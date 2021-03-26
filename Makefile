SOURCE = $(shell find . -name *.go -type f)
bldNum = $(if $(BLD_NUM),$(BLD_NUM),9999)
version = $(if $(VERSION),$(VERSION),1.0.0)
productVersion = $(version)-$(bldNum)
ARTIFACTS = build/artifacts/

# This allows the container tags to be explicitly set.
DOCKER_USER = couchbase
DOCKER_TAG = v1

build: $(SOURCE) go.mod
	for platform in linux darwin ; do \
	  echo "Building $$platform binary" ; \
	  GOOS=$$platform GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on go build -ldflags="-s -w" -o bin/$$platform/couchbase-watcher ; \
	done

image-artifacts: build
	mkdir -p $(ARTIFACTS)/bin/linux
	cp bin/linux/couchbase-watcher $(ARTIFACTS)/bin/linux
	cp Dockerfile* LICENSE README.md $(ARTIFACTS)
	cp -rv conf test redaction $(ARTIFACTS)

# This target (and only this target) is invoked by the production build job.
# This job will archive all files that end up in the dist/ directory.
dist: image-artifacts
	rm -rf dist
	mkdir -p dist
	tar -C $(ARTIFACTS) -czvf dist/couchbase-operator-logging-image_$(productVersion).tgz .
	rm -rf $(ARTIFACTS)

lint:
	docker run --rm -i hadolint/hadolint < Dockerfile 
	docker run --rm -i hadolint/hadolint < Dockerfile.rhel
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run ./...

# NOTE: This target is only for local development. While we use this Dockerfile
# (for now), the actual "docker build" command is located in the Jenkins job
# "couchbase-operator-docker". We could make use of this Makefile there as
# well, but it is quite possible in future that the canonical Dockerfile will
# need to be moved to a separate repo in which case the "docker build" command
# can't be here anyway.
container: build
	docker build -f Dockerfile -t ${DOCKER_USER}/operator-logging:${DOCKER_TAG} .

test: container
	docker build -f Dockerfile --target test -t ${DOCKER_USER}/operator-logging-test:${DOCKER_TAG} .
	docker run --rm -it ${DOCKER_USER}/operator-logging-test:${DOCKER_TAG}
