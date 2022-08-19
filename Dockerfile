ARG FLUENT_BIT_VER=1.9.7
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
COPY lua/sha1/ /usr/local/share/lua/5.1/sha1/
# Add our custom lua scripts
COPY lua/*.lua /fluent-bit/etc/

# Testing image to verify parsers and the watcher functionality
ARG FLUENT_BIT_VER=1.9.7
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
ENV	MBL_AUDIT=false MBL_ERLANG=false MBL_EVENTING=false MBL_HTTP=false MBL_INDEX_PROJECTOR=false MBL_JAVA=false MBL_MEMCACHED=false MBL_PROMETHEUS=false MBL_REBALANCE=false MBL_XDCR=false
# Kubernetes defaults
ENV POD_NAMESPACE=unknown POD_NAME=unknown POD_UID=unknown
# Couchbase defaults
ENV couchbase_cluster=unknown operator.couchbase.com/version=unknown server.couchbase.com/version=unknown couchbase_node=unknown couchbase_node_conf=unknown couchbase_server=unknown
# Service label defaults to false
ENV couchbase_service_analytics=false couchbase_service_data=false couchbase_service_eventing=false couchbase_service_index=false couchbase_service_query=false couchbase_service_search=false

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
ARG FLUENT_BIT_VER=1.9.7
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
ARG FLUENT_BIT_VER=1.9.7
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

# Disable mem buf limits by default
ENV	MBL_AUDIT=false MBL_ERLANG=false MBL_EVENTING=false MBL_HTTP=false MBL_INDEX_PROJECTOR=false MBL_JAVA=false MBL_MEMCACHED=false MBL_PROMETHEUS=false MBL_REBALANCE=false MBL_XDCR=false
# Kubernetes defaults
ENV POD_NAMESPACE=unknown POD_NAME=unknown POD_UID=unknown
# Couchbase defaults
ENV couchbase_cluster=unknown operator.couchbase.com/version=unknown server.couchbase.com/version=unknown couchbase_node=unknown couchbase_node_conf=unknown couchbase_server=unknown
# Service label defaults to false
ENV couchbase_service_analytics=false couchbase_service_data=false couchbase_service_eventing=false couchbase_service_index=false couchbase_service_query=false couchbase_service_search=false

# Entry point - run our custom binary
CMD ["/fluent-bit/bin/couchbase-watcher"]
