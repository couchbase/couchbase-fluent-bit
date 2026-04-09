# ------------------------------------------------------------------------------
# Stage 1: Production 
# Uses the dev variant (47MB) to enable apt-get for complex plugin dependencies.
# ------------------------------------------------------------------------------
FROM dhi.io/fluent-bit:4.2-dev AS production

ARG TARGETARCH
ARG PROD_VERSION

# Switch to root temporarily via its numeric ID (0)
USER 0

# Install required external plugin dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    libsasl2-2 \
    libpq5 \
    libcurl4 && \
    rm -rf /var/lib/apt/lists/*

# The dhi.io image puts fluent bit here. Update the variable so the watcher finds it.
ENV COUCHBASE_LOGS_BINARY=/usr/local/bin/fluent-bit

# Setup directories
RUN mkdir -p /fluent-bit/bin \
             /fluent-bit/etc/couchbase \
             /fluent-bit/config \
             /tmp/rebalance-logs \
             /opt/couchbase/var/lib/couchbase/logs \
             /usr/local/share/lua/5.1/sha1 && \
    chown -R 65532:65532 /fluent-bit /tmp /opt /usr/local/share/lua

# Layer on Couchbase watcher binary
COPY --chown=65532:65532 bin/linux/couchbase-watcher-${TARGETARCH} /fluent-bit/bin/couchbase-watcher

# Add default configurations
COPY --chown=65532:65532 config/conf/ /fluent-bit/etc/
COPY --chown=65532:65532 config/conf/fluent-bit.conf /fluent-bit/config/fluent-bit.conf

# Add custom lua scripts
COPY --chown=65532:65532 lua/sha1/ /usr/local/share/lua/5.1/sha1/
COPY --chown=65532:65532 lua/*.lua /fluent-bit/etc/

# ------------------------------------------------------------------------------
# Consolidated Environment Variables (Inherited by all future stages)
# ------------------------------------------------------------------------------
ENV COUCHBASE_LOGS_REBALANCE_TMP_DIR=/tmp/rebalance-logs \
    COUCHBASE_LOGS=/opt/couchbase/var/lib/couchbase/logs \
    COUCHBASE_AUDIT_LOGS=/opt/couchbase/var/lib/couchbase/logs \
    COUCHBASE_LOGS_DYNAMIC_CONFIG=/fluent-bit/config \
    COUCHBASE_LOGS_CONFIG_FILE=/fluent-bit/config/fluent-bit.conf \
    FLUENTBIT_VERSION=$FLUENT_BIT_VERSION \
    COUCHBASE_FLUENTBIT_VERSION=$PROD_VERSION \
    STDOUT_MATCH="couchbase.log.*" \
    FLUENT_BIT_LOG_LEVEL=info \
    LOKI_MATCH=no-match LOKI_HOST=loki LOKI_PORT=3100 LOKI_WORKERS=1 LOKI_TLS=OFF LOKI_TLS_VERIFY=OFF \
    ES_HOST=elasticsearch ES_PORT=9200 ES_INDEX=couchbase ES_MATCH=no-match ES_HTTP_USER=user ES_HTTP_PASSWD=password \
    SPLUNK_HOST=splunk SPLUNK_PORT=8088 SPLUNK_TOKEN=abcd1234 SPLUNK_MATCH=no-match SPLUNK_MATCH_REGEX=no-match SPLUNK_TLS=off SPLUNK_TLS_VERIFY=off SPLUNK_WORKERS=1 \
    MBL_AUDIT=false MBL_ERLANG=false MBL_EVENTING=false MBL_HTTP=false MBL_INDEX=false MBL_PROJECTOR=false MBL_JAVA=false MBL_MEMCACHED=false MBL_PROMETHEUS=false MBL_REBALANCE=false MBL_XDCR=false MBL_QUERY=false MBL_FTS=false \
    POD_NAMESPACE=unknown POD_NAME=unknown POD_UID=unknown \
    couchbase_cluster=unknown operator.couchbase.com/version=unknown server.couchbase.com/version=unknown couchbase_node=unknown couchbase_node_conf=unknown couchbase_server=unknown \
    couchbase_service_analytics=false couchbase_service_data=false couchbase_service_eventing=false couchbase_service_index=false couchbase_service_query=false couchbase_service_search=false

# Drop privileges permanently to the CIS-compliant non-root user
USER 65532

VOLUME /fluent-bit/config

ENTRYPOINT ["/fluent-bit/bin/couchbase-watcher"]


# ------------------------------------------------------------------------------
# Stage 2: Test 
# ------------------------------------------------------------------------------
FROM production AS test

ARG TARGETARCH

# Switch to root temporarily via its numeric ID (0)
USER 0

# 1. Copy the test files and binaries FIRST
COPY --chown=65532:65532 test/ /fluent-bit/test/
COPY --chown=65532:65532 bin/linux/log-differ-${TARGETARCH} /bin/log-differ

# 2. Setup test directories and apply broad permissions for the testing framework
RUN mkdir -p /fluent-bit/test/logs/rebalance-logs /tmp/buffers && \
    chmod 1777 /tmp/buffers && \
    chmod 777 -R /fluent-bit/test /fluent-bit/etc/couchbase

# Drop privileges back to the non-root user
USER 65532

# Override specific variables just for the testing environment
ENV COUCHBASE_LOGS=/fluent-bit/test/logs \
    COUCHBASE_AUDIT_LOGS=/fluent-bit/test/logs \
    COUCHBASE_LOGS_REBALANCE_TMP_DIR=/fluent-bit/test/logs/rebalance-logs

# Clear the inherited ENTRYPOINT from Stage 1 so our test script can run independently
ENTRYPOINT []

CMD ["/bin/bash", "/fluent-bit/test/run-tests.sh"]


# ------------------------------------------------------------------------------
# Stage 3: Un-targeted Final Build (Pipeline Support)
# ------------------------------------------------------------------------------
FROM production

LABEL description="Couchbase Fluent Bit image with support for config reload, pre-processing and redaction" \
      vendor="Couchbase" \
      maintainer="docker@couchbase.com"

# Switch to root temporarily via its numeric ID (0)
USER 0

COPY --chown=65532:65532 licenses/* /licenses/
COPY --chown=65532:65532 README.md /help.1

# Drop privileges permanently to the CIS-compliant non-root user
USER 65532

ARG HTTP_PORT=2020
ENV HTTP_PORT=$HTTP_PORT
EXPOSE $HTTP_PORT