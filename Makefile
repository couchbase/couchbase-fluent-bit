SOURCE = $(shell find . -name *.go -type f)
bldNum = $(if $(BLD_NUM),$(BLD_NUM),9999)
version = $(if $(VERSION),$(VERSION),1.0.0)
productVersion = $(version)-$(bldNum)
ARTIFACTS = build/artifacts/

# This allows the container tags to be explicitly set.
DOCKER_USER = couchbase
DOCKER_TAG = v1

.PHONY: all build lint test-unit container container-rhel container-public container-lint container-scan container-rhel-checks dist test test-dist container-clean clean

all: clean build lint test-unit container container-rhel container-lint container-scan container-rhel-checks test dist test-dist

build: $(SOURCE) go.mod
	for platform in linux darwin ; do \
	  echo "Building $$platform binary" ; \
	  GOOS=$$platform GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on go build -ldflags="-s -w" -o bin/$$platform/couchbase-watcher ./cmd ; \
	done

image-artifacts: build
	mkdir -p $(ARTIFACTS)/bin/linux
	cp bin/linux/couchbase-watcher $(ARTIFACTS)/bin/linux
	cp Dockerfile* LICENSE README.md $(ARTIFACTS)
	cp -rv conf licenses redaction test $(ARTIFACTS)

# This target (and only this target) is invoked by the production build job.
# This job will archive all files that end up in the dist/ directory.
dist: image-artifacts
	rm -rf dist
	mkdir -p dist
	tar -C $(ARTIFACTS) -czvf dist/couchbase-fluent-bit-image_$(productVersion).tgz .
	rm -rf $(ARTIFACTS)

lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run ./cmd/... ./pkg/...

test-unit:
	go test -timeout 30s -v ./pkg/...

# NOTE: This target is only for local development. While we use this Dockerfile
# (for now), the actual "docker build" command is located in the Jenkins job
# "couchbase-operator-docker". We could make use of this Makefile there as
# well, but it is quite possible in future that the canonical Dockerfile will
# need to be moved to a separate repo in which case the "docker build" command
# can't be here anyway.
container: build
	docker build -f Dockerfile -t ${DOCKER_USER}/fluent-bit:${DOCKER_TAG} .
	docker build -f Dockerfile --target test -t ${DOCKER_USER}/fluent-bit-test:${DOCKER_TAG} .

container-rhel: build
	docker build -f Dockerfile.rhel --build-arg OPERATOR_BUILD=$(OPERATOR_BUILD) --build-arg OS_BUILD=$(BUILD) --build-arg PROD_VERSION=$(VERSION) -t ${DOCKER_USER}/fluent-bit-rhel:${DOCKER_TAG} .
	docker build -f Dockerfile.rhel --build-arg OPERATOR_BUILD=$(OPERATOR_BUILD) --build-arg OS_BUILD=$(BUILD) --build-arg PROD_VERSION=$(VERSION) --target test -t ${DOCKER_USER}/fluent-bit-test-rhel:${DOCKER_TAG} .

container-lint: build lint
	docker run --rm -i hadolint/hadolint < Dockerfile 
	docker run --rm -i -v $(CURDIR)/.hadolint.yaml.rhel:/root/.config/hadolint.yaml:ro hadolint/hadolint < Dockerfile.rhel

container-scan: container container-rhel
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy \
		--severity "HIGH,CRITICAL" --ignore-unfixed --exit-code 1 --no-progress ${DOCKER_USER}/fluent-bit:${DOCKER_TAG}
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy \
		--severity "HIGH,CRITICAL" --ignore-unfixed --exit-code 1 --no-progress ${DOCKER_USER}/fluent-bit-rhel:${DOCKER_TAG}
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock -e CI=true wagoodman/dive \
		${DOCKER_USER}/fluent-bit:${DOCKER_TAG}
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock -v $(CURDIR)/.dive.ci.rhel:/root/.config/dive.ci:ro -e CI=true wagoodman/dive \
		--ci-config /root/.config/dive.ci ${DOCKER_USER}/fluent-bit-rhel:${DOCKER_TAG}

# Check for vulnerabilites and some of the requirements of Red Hat certification
# Licenses provided
# Layer count <40
# Goal here is to fail early with some simple checks
container-rhel-checks: container-scan
	docker run --rm -it ${DOCKER_USER}/fluent-bit-rhel:${DOCKER_TAG} ls -l /licenses/
	test $(shell docker image history -q ${DOCKER_USER}/fluent-bit-rhel:${DOCKER_TAG}| wc -l) -lt 40
	docker save -o fluent-bit-rhel.tar ${DOCKER_USER}/fluent-bit-rhel:${DOCKER_TAG}
	go run github.com/heroku/terrier -cfg terrier.cfg.yml && rm -f fluent-bit-rhel.tar

test: test-unit container container-rhel container-lint
	docker run --rm -it ${DOCKER_USER}/fluent-bit-test:${DOCKER_TAG}
	docker run --rm -it ${DOCKER_USER}/fluent-bit-test-rhel:${DOCKER_TAG}

# This target pushes the containers to a public repository.
# A typical one liner to deploy to the cloud would be:
# 	make container-public -e DOCKER_USER=couchbase DOCKER_TAG=2.0.0
container-public: container
	docker push ${DOCKER_USER}/fluent-bit:${DOCKER_TAG}
	docker push ${DOCKER_USER}/fluent-bit-test:${DOCKER_TAG}

# Special target to verify the internal release pipeline will work as well
# Take the archive we would make and extract it to a local directory to then run the docker builds on
test-dist: dist
	rm -rf test-dist/
	mkdir -p test-dist/
	tar -xzvf dist/couchbase-fluent-bit-image_$(productVersion).tgz -C test-dist/
	docker build -f test-dist/Dockerfile test-dist/ -t ${DOCKER_USER}/fluent-bit-test-dist:${DOCKER_TAG}
	docker build -f test-dist/Dockerfile.rhel test-dist/ -t ${DOCKER_USER}/fluent-bit-test-dist-rhel:${DOCKER_TAG}

container-clean:
	docker rmi -f ${DOCKER_USER}/fluent-bit:${DOCKER_TAG} \
				  ${DOCKER_USER}/fluent-bit-test:${DOCKER_TAG} \
				  ${DOCKER_USER}/fluent-bit-rhel:${DOCKER_TAG} \
				  ${DOCKER_USER}/fluent-bit-test-rhel:${DOCKER_TAG} \
				  ${DOCKER_USER}/fluent-bit-test-dist:${DOCKER_TAG} \
				  ${DOCKER_USER}/fluent-bit-test-dist-rhel:${DOCKER_TAG}

clean: container-clean
	rm -rf $(ARTIFACTS) bin/ dist/ test-dist/
