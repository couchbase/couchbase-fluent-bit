FROM golang:1.16.0-alpine3.13 as buildergo
RUN mkdir -p /fluent-bit
WORKDIR /fluent-bit

# Copy over source
COPY main.go go.mod go.sum ./
ENV CGO_ENABLED=0 
RUN go build -ldflags '-w -s' -o couchbase-watcher main.go

FROM golangci/golangci-lint:v1.38 AS lint-base

FROM buildergo as lint
COPY --from=lint-base /usr/bin/golangci-lint /usr/bin/golangci-lint
RUN golangci-lint run --timeout 10m0s ./...

FROM fluent/fluent-bit:1.7.2 as production
# hadolint ignore=DL3048
LABEL description="Couchbase Fluent Bit image with support for config reload, pre-processing and redaction" vendor="Couchbase" maintainer="docker@couchbase.com"

# We need to layer on a binary to pre-process the rebalance reports and watch for config changes
COPY --from=buildergo /fluent-bit/couchbase-watcher /fluent-bit/bin/couchbase-watcher
ENV COUCHBASE_LOGS_REBALANCE_TEMPDIR /tmp/rebalance-logs
# Default location for logs but also set by the operator
ENV COUCHBASE_LOGS /opt/couchbase/var/couchbase/logs
# Add any default configuration we can provide
COPY conf/ /fluent-bit/etc/

# To support mounting a configmap or secret but mixing with existing files we use a separate volume
# This way we can keep using the parsers defined without having to re-define them.
# If we try to mount via sub-path then it won't update: https://github.com/kubernetes/kubernetes/issues/50345
VOLUME /fluent-bit/config
ENV FLUENT_BIT_DYNAMIC_CONFIG /fluent-bit/config
# Put a copy of the config in the area we want to monitor
COPY conf/fluent-bit.conf /fluent-bit/config/fluent-bit.conf
# Entry point - run our custom binary
CMD ["/fluent-bit/bin/couchbase-watcher", "-c", "/fluent-bit/config/fluent-bit.conf"]

# Add support for SHA1 hashing via a pure LUA implementation to use in redaction tutorial
COPY redaction/sha1/ /usr/local/share/lua/5.1/sha1/
# Add our custom redaction script
COPY redaction/redaction.lua /fluent-bit/etc/

# Copying the base image to expose for the HTTP server if enabled
EXPOSE 2020

FROM fluent/fluent-bit:1.7.2-debug as test

COPY --from=production /fluent-bit/ /fluent-bit/
# Add support for SHA1 hashing via a pure LUA implementation to use in redaction tutorial
COPY redaction/sha1/ /usr/local/share/lua/5.1/sha1/

COPY test/ /fluent-bit/test/

# Redirect to local logs
ENV COUCHBASE_LOGS /fluent-bit/test/logs
ENV COUCHBASE_LOGS_REBALANCE_TEMPDIR /fluent-bit/test/logs/rebalance-logs

VOLUME /logs

# Wrap our test cases in a script that supports checking for errors and then using an exit code directly
# https://github.com/fluent/fluent-bit/issues/3268
# It can also run all test cases in one go then rather than have to list them all individually
CMD ["sh", "/fluent-bit/test/run-tests.sh"]