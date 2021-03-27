# couchbase-operator-logging

## Summary

The Couchbase Operator Logging image is an image based on the official [Fluent Bit](https://fluentbit.io/) [image](https://hub.docker.com/r/fluent/fluent-bit/) with some additional support for the following:
1. Dynamic configuration reload - changes to the configuration are watched for and when detected trigger a restart of Fluent Bit to pick up the new configuration.
2. Rebalace report pre-processing - the rebalance reports produced by Couchbase need some additional pre-processing before they can be parsed by Fluent Bit.
3. SHA1 LUA hashing implementation and redaction support included (but not enabled by default).

This image is intended to be used as a sidecar with a Couchbase Autonomous Operator deployment to automatically ship various couchbase logs.
The log shipping can be dynmically configured per namespace/Couchbase cluster via a standard Kubernetes secret which is mounted then as the configuration directory.

The image could also be used with an on-premise deployment to ship local logs in the same fashion although this is not currently tested.

To help provide the capability in this image we make use of other OSS:
* https://github.com/kubesphere/fluent-bit
* https://github.com/mpeterv/sha1
* https://github.com/fluent/fluent-bit 

### Logs supported

Refer to the [official documentation](https://docs.couchbase.com/server/current/manage/manage-logging/manage-logging.html#log-file-listing) for all possible logs output by Couchbase Server.

This image is used to parse and send the following logs to standard output by default:
* analytics_debug.log
* audit.log
* babysitter.log
* couchdb.log
* debug.log
* eventing.log
* fts.log
* goxdcr.log
* http_access.log
* http_access_internal.log
* indexer.log
* json_rpc.log
* memcached.log
* metakv.log
* ns_couchdb.log
* projector.log
* Rebalance reports
* reports.log

It will also handle the logs like `info.log`, `error.log` that are a subset of the full `debug.log` - there is no point parsing all these logs as it will duplicate the information.

The definition of "parse" here means to turn the unstructured, possibly multi-line log output into structured data we can filter, mutate & forward to any supported Fluent Bit endpoint. 
For the purposes of this image, we essentially chunk up the log lines into timestamp, level and message - message can be over multiple lines.

Every log is tagged individually to form its own stream within the log shipper.
The tag format is `couchbase.log.<name>`. 
Each stream can be managed independently so refer to the official Fluent Bit documentation for full details on the extensive capability and configuration available.

Other logs than the list above may be supported by the provided parsers as well.

## Technical Overview

This image is essentially the official Fluent Bit image with an entrypoint watcher process that handles either restarting Fluent Bit on config change or pre-processing the rebalance reports.

### Parsing
For the purposes of this implementation, parsing is very simple as we’re not trying to extract any more information than the simple timestamp, log level and log message. 
A few of the logs (http and JSON format ones - audit & rebalance reports) can have some extra fields extracted as it is straightforward but any significant processing is left for the consumer to manage.

For those logs with multiline output, the parser should capture everything up to the next log statement. In some cases this includes large content (e.g. Java thread dumps) but this is all treated as part of the log message for the consumer to work with.

The parsers are provided in `conf/parsers-couchbase.conf` along with default configuration for each log file in `conf/couchbase/in-*.conf`.

### Rebalance reports and dynamic configuration
The official version of Fluent Bit does not support dynamic changes to its configuration: if you change the log shipping configuration then you have to restart it. 
We cannot restart the pods without triggering rebalance.

The rebalance reports have a bit of an issue with the default tail plugin: they are a file with no new lines. 
The full JSON dump of the report is all over a single line which can be quite large (not for a log file but for a single line in a log file). 
The tail plugin works on a per-line basis so cannot handle the reports as they currently are. 
Additionally there is the question of which timestamp to use as the “log” timestamp - a rebalance report can have multiple ones using common tags.

We have solved both these problems by forking the Kubesphere solution (a fork from the official image) to resolve the dynamic configuration issue. 
This watches for config file changes and then restarts Fluent Bit to pick up the new configuration but all within the container. 
We can extend this to handle the rebalance reports: we watch for them, when we see one we copy it to a temporary location with any pre-processing we want done and then Fluent Bit reads the copy. 
We copy it for these reasons rather than updating in place:
1. No change to the original log or anything reliant on it (e.g. rotation).
2. The logging sidecar deliberately has no write access to the log volume so it cannot modify any logs. 
3. The log timestamp can be collected from the rebalance report name which includes the time it was created.

Whilst Fluent Bit is restarting, no logs will be shipped out of the container. 
We could re-parse logs but this would then lead to duplicate entries from previously parsed logs. 
The intention is that reconfiguration is an asynchronous un-common operation so the temporary potential loss of logs is acceptable.

Interestingly as part of this work, we discovered that the `exec` plugin is not usable in a container without `/bin/sh`, i.e. all the official ones including the debug variant with busybox. 
The original intention was to use this to process the log file but that was impossible: even with a compiled binary it still must be invoked via `/bin/sh`.

### Redaction
Log redaction in flight has been demonstrated and is tested but will not be provided by default. 

A tutorial is provided on how to configure this if required so refer to the Couchbase Autonomous Operator documentation for that.
There may be a performance impact to redaction in flight and it will also complicate debugging of problems if the logs are auto-redacted within the cluster.

To simplify usage we build everything required into the container image: this is a distroless image so has no OS support for hashing out of the box. 
we therefore include the LUA implementation from: https://github.com/mpeterv/sha1 

Similarly any other log mutation could be done that is supported by Fluent Bit. 
Using a LUA script provides a lot of flexibility but there are plenty of other simpler plugins to modify the content or destination of a log. 
The recommendation when using LUA parsing is to dedicate a worker thread to it.

### Specific parsers

Some of the logs (FTS and eventing in the default configuration) provide a mixed timestamp output which is difficult to parse in one go.
Instead the timestamps are extracted via a generic parser and then run through an additional stage to parse it in the appropriate format.
This may not be required if it is acceptable to use local time, i.e. the time at which Fluent Bit tails that line in the log.

## Usage

The official Couchbase Autonomous Operator documentation provides full details on using this with Kubernetes including additional tutorials on consuming logs with Loki or sending to Azure, S3, etc.

The image is basically identical to the official Fluent Bit image and can be used in the same fashion. 
If the capabilities listed above are not required then the official Fluent Bit image can be used as well.

This image is only intended to be used with Kubernetes although it may be usable as a standalone Docker image either for a containerised or on-premise deployment but this is not an officially supported configuration.

| Environment variable | Description | Default |
| --- | --- | --- |
| COUCHBASE_LOGS | The directory in which to find the various Couchbase logs we are interested in | /opt/couchbase/var/couchbase/logs | 
| COUCHBASE_LOGS_REBALANCE_TEMPDIR | The temporary directory for out pre-processed rebalance reports | /tmp/rebalance-logs |

## Building

This repository is set up to be built by the internal Couchbase process with a `Makefile`.
This can be easily reused though to build locally either by installing the relevant tools or using a Golang container to build the source.

## Testing

A set of automated tests are provided to verify changes against for sanity checks and some regression testing too of known input and expected output.
New sets of input data can be used as well by running the container with the logs (input) and expected output using a volume mount:
```
docker run --rm -it --mount type=bind,source=<directory>,target=/fluent-bit/tests/logs <test container name>
```
The directory should be made up of matching pairs of logs and expected output in the following format: `<name>.log` --> `<name>.log.expected`.
The container will run the Couchbase Fluent Bit image against each log file in turn with some basic sanity checks and produce an output file named `<name>.log.actual`.
The `tests/run-tests.sh` script will then iterate over all expected output to compare it against actual output.

To help with verifying new output, a simple NodeJS tool is provided in `tools/log-verifier`. 
This can be run locally or as an image against the files to check, when run as an image the file will need mounting into the container and passing as an argument.

For the RHEL variant we make best effort to verify Fluent Bit is working using its unit tests however the only supported usage is of the `tail` input plugin to `stdout` output plugin pipeline used in the default configuration for the Couchbase Autonomous Operator. 

## Reporting Bugs and Issues
Please use our official [JIRA board](https://issues.couchbase.com/projects/K8S/issues/?filter=allopenissues) to report any bugs and issues.

## License

Copyright 2021 Couchbase Inc.

Licensed under the Apache License, Version 2.0

See [LICENSE](https://github.com/couchbase/couchbase-operator-logging/blob/master/LICENSE) for further details.