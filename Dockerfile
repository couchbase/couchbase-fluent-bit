# Intermediate image used as a pre-cursor to testing and the final released image
# Have to use a fixed base image for the build framework
ARG FLUENT_BIT_VER=1.8.7
FROM fluent/fluent-bit:$FLUENT_BIT_VER as production

ENV COUCHBASE_LOGS_BINARY /fluent-bit/bin/fluent-bit

# We need to layer on a binary to pre-process the rebalance reports and watch for config changes
COPY bin/linux/couchbase-watcher /fluent-bit/bin/couchbase-watcher
# Add any default configuration we can provide
COPY conf/ /fluent-bit/etc/

# Set up output for rebalance pre-processing - can be overridden, e.g. for testing
ENV COUCHBASE_LOGS_REBALANCE_TEMPDIR /tmp/rebalance-logs
# Default location for logs but also set by the operator
ENV COUCHBASE_LOGS /opt/couchbase/var/lib/couchbase/logs

# To support mounting a configmap or secret but mixing with existing files we use a separate volume
# This way we can keep using the parsers defined without having to re-define them.
# If we try to mount via sub-path then it won't update: https://github.com/kubernetes/kubernetes/issues/50345
VOLUME /fluent-bit/config
ENV COUCHBASE_LOGS_DYNAMIC_CONFIG /fluent-bit/config
# Put a copy of the config in the area we want to monitor
COPY conf/fluent-bit.conf /fluent-bit/config/fluent-bit.conf
ENV COUCHBASE_LOGS_CONFIG_FILE /fluent-bit/config/fluent-bit.conf

# Add support for SHA1 hashing via a pure LUA implementation to use in redaction tutorial
COPY redaction/sha1/ /usr/local/share/lua/5.1/sha1/
# Add our custom redaction script
COPY redaction/redaction.lua /fluent-bit/etc/

# Testing image to verify parsers and the watcher functionality
ARG FLUENT_BIT_VER=1.8.7
FROM fluent/fluent-bit:$FLUENT_BIT_VER-debug as test
ENV COUCHBASE_LOGS_BINARY /fluent-bit/bin/fluent-bit

COPY --from=production /fluent-bit/ /fluent-bit/
# Add support for SHA1 hashing via a pure LUA implementation to use in redaction tutorial
COPY redaction/sha1/ /usr/local/share/lua/5.1/sha1/
# Add test cases (need write permissions as well for logs)
COPY test/ /fluent-bit/test/

# Redirect to local logs
ENV COUCHBASE_LOGS /fluent-bit/test/logs
ENV COUCHBASE_LOGS_REBALANCE_TEMPDIR /fluent-bit/test/logs/rebalance-logs

# Use busybox so custom shell location, need to chmod for log output write access
SHELL [ "/usr/local/bin/sh", "-c" ]
RUN chmod 777 /fluent-bit/test/ && \
    chmod 777 /fluent-bit/test/logs && \
    chmod 777 /fluent-bit/etc/couchbase

# Ensure we run as non-root by default
COPY non-root.passwd /etc/passwd
USER 8453

# Copying the base image to expose for the HTTP server if enabled
EXPOSE 2020

# Keep track of the versions we are using - not persisted between stages
ARG FLUENT_BIT_VER=1.8.7
ENV FLUENTBIT_VERSION=$FLUENT_BIT_VER
ARG PROD_VERSION
ENV COUCHBASE_FLUENTBIT_VERSION=$PROD_VERSION

# Wrap our test cases in a script that supports checking for errors and then using an exit code directly
# https://github.com/fluent/fluent-bit/issues/3268
# It can also run all test cases in one go then rather than have to list them all individually
CMD ["sh", "/fluent-bit/test/run-tests.sh"]

# We need an un-targeted build stage to support the build pipeline
FROM production
LABEL description="Couchbase Fluent Bit image with support for config reload, pre-processing and redaction" vendor="Couchbase" maintainer="docker@couchbase.com"
# Ensure we include any relevant docs
COPY licenses/* /licenses/
COPY README.md /help.1

# Copying the base image to expose for the HTTP server if enabled
EXPOSE 2020

# Ensure we run as non-root by default
COPY non-root.passwd /etc/passwd
USER 8453

# Keep track of the versions we are using - not persisted between stages
ARG FLUENT_BIT_VER=1.8.7
ENV FLUENTBIT_VERSION=$FLUENT_BIT_VER
ARG PROD_VERSION
ENV COUCHBASE_FLUENTBIT_VERSION=$PROD_VERSION

# Entry point - run our custom binary
CMD ["/fluent-bit/bin/couchbase-watcher"]
