ARG FLUENT_BIT_VER=1.8.14
FROM debian:bullseye-slim as deb-extractor
# We download all debs locally then extract them into a directory we can use as the root for distroless.
# We also include some extra handling for the status files that some tooling uses for scanning, etc.
WORKDIR /tmp
#hadolint ignore=DL4006,SC2086
RUN apt-get update && \
    apt-get download \
        libssl1.1 \
        libsasl2-2 \
        pkg-config \
        libpq5 \
        libsystemd0 \
        zlib1g \
        ca-certificates \
        libatomic1 \
        libgcrypt20 \
        libzstd1 \
        liblz4-1 \
        libgssapi-krb5-2 \
        libldap-2.4-2 \
        libgpg-error0 \
        libkrb5-3 \
        libk5crypto3 \
        libcom-err2 \
        libkrb5support0 \
        libgnutls30 \
        libkeyutils1 \
        libp11-kit0 \
        libidn2-0 \
        libunistring2 \
        libtasn1-6 \
        libnettle8 \
        libhogweed6 \
        libgmp10 \
        libffi7 \
        liblzma5 \
        libyaml-0-2 && \
    mkdir -p /dpkg/var/lib/dpkg/status.d/ && \
    for deb in *.deb; do \
        package_name=$(dpkg-deb -I ${deb} | awk '/^ Package: .*$/ {print $2}'); \
        echo "Processing: ${package_name}"; \
        dpkg --ctrl-tarfile $deb | tar -Oxf - ./control > /dpkg/var/lib/dpkg/status.d/${package_name}; \
        dpkg --extract $deb /dpkg || exit 10; \
    done

# Remove unnecessary files extracted from deb packages like man pages and docs etc.
RUN find /dpkg/ -type d -empty -delete && \
    rm -r /dpkg/usr/share/doc/

# Intermediate image used as a pre-cursor to testing and the final released image
# Have to use a fixed base image for the build framework
ARG FLUENT_BIT_VER=1.8.14
FROM fluent/fluent-bit:$FLUENT_BIT_VER as builder

#hadolint ignore=DL3006
FROM gcr.io/distroless/cc-debian11 as production

COPY --from=deb-extractor /dpkg /
COPY --from=builder /fluent-bit /fluent-bit

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
COPY lua/sha1/ /usr/local/share/lua/5.1/sha1/
# Add our custom lua scripts
COPY lua/*.lua /fluent-bit/etc/

# Testing image to verify parsers and the watcher functionality
ARG FLUENT_BIT_VER=1.8.14
FROM fluent/fluent-bit:${FLUENT_BIT_VER}-debug as test
ENV COUCHBASE_LOGS_BINARY /fluent-bit/bin/fluent-bit

COPY --from=production /fluent-bit/ /fluent-bit/
# Add support for SHA1 hashing via a pure LUA implementation to use in redaction tutorial
COPY lua/sha1/ /usr/local/share/lua/5.1/sha1/

# Add our custom lua scripts
COPY lua/*.lua /fluent-bit/etc/
# Add test cases (need write permissions as well for logs)
COPY test/ /fluent-bit/test/
# Copy over the log differ binary
COPY bin/linux/log-differ /bin/log-differ

# Redirect to local logs
ENV COUCHBASE_LOGS /fluent-bit/test/logs
ENV COUCHBASE_LOGS_REBALANCE_TEMPDIR /fluent-bit/test/logs/rebalance-logs

# Disable mem buf limits for testing
ENV	MBL_AUDIT "false"
ENV	MBL_ERLANG "false"
ENV	MBL_EVENTING "false"
ENV	MBL_HTTP "false"
ENV	MBL_INDEX_PROJECTOR "false"
ENV	MBL_JAVA "false"
ENV	MBL_MEMCACHED "false"
ENV	MBL_PROMETHEUS "false"
ENV	MBL_REBALANCE "false"
ENV	MBL_XDCR "false"

# Use busybox so custom shell location, need to chmod for log output write access

RUN chmod 777 /fluent-bit/test/ && \
    chmod 777 /fluent-bit/test/logs && \
    chmod 777 /fluent-bit/etc/couchbase

# Create folder for input plugin buffers
RUN mkdir /tmp/buffers && \
    chmod 1777 /tmp/buffers

# Ensure we run as non-root by default
COPY non-root.passwd /etc/passwd
USER 8453

# Copying the base image to expose for the HTTP server if enabled
ARG HTTP_PORT=2020
ENV HTTP_PORT=$HTTP_PORT
EXPOSE $HTTP_PORT

# Keep track of the versions we are using - not persisted between stages
ARG FLUENT_BIT_VER=1.8.14
ENV FLUENTBIT_VERSION=$FLUENT_BIT_VER
ARG PROD_VERSION
ENV COUCHBASE_FLUENTBIT_VERSION=$PROD_VERSION

# Wrap our test cases in a script that supports checking for errors and then using an exit code directly
# https://github.com/fluent/fluent-bit/issues/3268
# It can also run all test cases in one go then rather than have to list them all individually
CMD ["/fluent-bit/test/run-tests.sh"]

# We need an un-targeted build stage to support the build pipeline
FROM production
LABEL description="Couchbase Fluent Bit image with support for config reload, pre-processing and redaction" vendor="Couchbase" maintainer="docker@couchbase.com"
# Ensure we include any relevant docs
COPY licenses/* /licenses/
COPY README.md /help.1

# Copying the base image to expose for the HTTP server if enabled
ARG HTTP_PORT=2020
ENV HTTP_PORT=$HTTP_PORT
EXPOSE $HTTP_PORT

# Ensure we run as non-root by default
COPY non-root.passwd /etc/passwd
USER 8453

# Keep track of the versions we are using - not persisted between stages
ARG FLUENT_BIT_VER=1.8.14
ENV FLUENTBIT_VERSION=$FLUENT_BIT_VER
ARG PROD_VERSION
ENV COUCHBASE_FLUENTBIT_VERSION=$PROD_VERSION

# The default match we send to standard output
ENV STDOUT_MATCH="couchbase.log.*"

# Some support for Loki customisation but ensure we set defaults
ENV LOKI_MATCH=no-match LOKI_HOST=loki LOKI_PORT=3100
# Elasiticsearch defaults
ENV ES_HOST=elasticsearch ES_PORT=9200 ES_INDEX=couchbase ES_MATCH=no-match ES_HTTP_USER="" ES_HTTP_PASSWD=""
# Splunk defaults
ENV SPLUNK_HOST=splunk SPLUNK_PORT=8088 SPLUNK_TOKEN=abcd1234 SPLUNK_MATCH=no-match

# Entry point - run our custom binary
CMD ["/fluent-bit/bin/couchbase-watcher"]
