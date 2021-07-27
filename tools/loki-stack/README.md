# Example Loki Stack with Couchbase Server

The contents of this directory are to support this blog post: https://blog.couchbase.com/using-fluent-bit-for-log-forwarding-processing-with-couchbase-server/

This directory includes everything to run a simple stack up locally using [Docker Compose](https://docs.docker.com/compose/) that supports log forwarding from a Couchbase Server instance to [Loki](https://grafana.com/oss/loki/) and then visualising in [Grafana](https://grafana.com/). This documentation also talks you how to repeat it all by hand.

Whilst the Couchbase Fluent Bit container image is only officially supported with the Couchbase Autonomous Operator on Kubernetes, it can still be used in a non-Kubernetes or on-premise deployment. Fluent Bit started life as a native log forwarding solution for embedded targets after all.

## Pre-requisites
* Docker: https://docs.docker.com/engine/install/
* Docker Compose: https://docs.docker.com/compose/install/

## Usage

The Couchbase Server container is purely run as an example to generate the logs. If you have a running server already then just use the directory it has for its logs in the [Log Forwarding](#log-forwarding) section.

To run, simply do a `docker-compose up` from this directory to start everything up.

To change any image versions this can be specified by the environment variables referenced in the Docker Compose [file](docker-compose.yml) with defaults in the environment [file](.env).

## Architecture

The whole example covers:
* Couchbase Server with bind mount for log directory - this is a preconfigured instance by default.
* Couchbase Fluent Bit with read-only bind mount for log directory and custom configuration to send to Loki.
* Loki to receive the logs.
* Grafana preconfigured to talk to Loki.

We will now go over how to replicate it with some additional information.

### Couchbase Server

Ignore this section if you have a deployed instance already or will do: if it is a containerised version just make sure the log directory is exposed via a [volume](https://docs.docker.com/storage/volumes/) or [bind mount](https://docs.docker.com/storage/bind-mounts/) on the host (we do this here).

Follow the [guidance](https://docs.couchbase.com/server/current/install/getting-started-docker.html) to deploy a set of one of more containers as a Couchbase cluster but with a volume for the logs:

```
mkdir -p /tmp/couchbase-logs
docker run --rm -d --name db -p 8091-8096:8091-8096 -p 11210-11211:11210-11211 -v /tmp/couchbase-logs/:/opt/couchbase/var/lib/couchbase/logs/ couchbase:6.6.2
```

The main thing here is to make sure we expose the logs produced by Couchbase Server so another container can pick them up otherwise they would all be isolated inside the container. **A native binary deployment of Couchbase Server would just be writing to a directory on the host so this is not required.**

The location of the log directory (inside the container or on the host) is part of the Couchbase Server [documentation](https://docs.couchbase.com/server/current/manage/manage-logging/manage-logging.html#log-file-listing).

Note that this uses a named container called `db` so make sure one does not already exist otherwise it will generate an error due to the conflict: `docker rm db`. Our command above automatically cleans up the container on exit.

Now, we can check logs are being created in our directory:

```
ls -l /tmp/couchbase-logs
total 1344
-rw-r-----  1 patrickstephens  wheel   23330  4 Jun 11:11 babysitter.log
-rw-r-----  1 patrickstephens  wheel     152  4 Jun 11:11 couchdb.log
-rw-r-----  1 patrickstephens  wheel  330926  4 Jun 11:11 debug.log
-rw-r-----  1 patrickstephens  wheel       0  4 Jun 11:11 error.log
-rw-r-----  1 patrickstephens  wheel    6926  4 Jun 11:11 goxdcr.log
-rw-r-----  1 patrickstephens  wheel       0  4 Jun 11:11 http_access.log
-rw-r-----  1 patrickstephens  wheel    2073  4 Jun 11:11 http_access_internal.log
-rw-r-----  1 patrickstephens  wheel   73345  4 Jun 11:11 info.log
-rw-r-----  1 patrickstephens  wheel    1928  4 Jun 11:11 json_rpc.log
-rw-r-----  1 patrickstephens  wheel       0  4 Jun 11:11 mapreduce_errors.log
-rw-r-----  1 patrickstephens  wheel    5303  4 Jun 11:11 memcached.log.000000.txt
-rw-r-----  1 patrickstephens  wheel    1389  4 Jun 11:11 metakv.log
-rw-r-----  1 patrickstephens  wheel   67103  4 Jun 11:11 ns_couchdb.log
drwxr-x---  2 patrickstephens  wheel      64  4 Jun 11:11 rebalance
-rw-r-----  1 patrickstephens  wheel  144264  4 Jun 11:11 reports.log
-rw-r-----  1 patrickstephens  wheel    4803  4 Jun 11:11 stats.log
-rw-r-----  1 patrickstephens  wheel       0  4 Jun 11:11 views.log
-rw-r-----  1 patrickstephens  wheel       0  4 Jun 11:11 xdcr_target.log
```

For the purposes of this deployment we do not really need to actually configure the cluster but it is useful to do so in order to get proper logs and data in there. Follow the [instructions](https://docs.couchbase.com/server/current/install/getting-started-docker.html) in the official documentation to configure the cluster via the UI and import some sample data as well if you want to.

If you want to run multiple containers on the same node to simulate a multi-node Couchbase Server cluster then make sure to use a separate log directory (or volume) for each container. Then run a Couchbase Fluent Bit image per Couchbase Server container mounting each directory as per the next section on Log Forwarding.

### Log forwarding

Once we have configured the cluster and optionally added some buckets, sample data, etc. we can run up the Fluent Bit container. This is fairly simple when using the Couchbase Fluent Bit image:

```
docker run --rm -d --name logger -v /tmp/couchbase-logs/:/opt/couchbase/var/lib/couchbase/logs/:ro -e COUCHBASE_LOGS=/opt/couchbase/var/lib/couchbase/logs/ couchbase/fluent-bit:1.0.1
```

As you can see here we mount our local directory and specify it as an environment variable (so you can mount it into another location in the container and just point it at that). This is how you can use it with a natively deployed Couchbase Server: mount the local directory for logs into the container instead of the temporary directory used in this example.

For the specific details of where Couchbase Server stores its logs refer to the [official documentation](https://docs.couchbase.com/server/current/manage/manage-logging/manage-logging.html#log-file-locations). This base directory is the one that would need mounting into the Couchbase Fluent Bit image as above. Be aware of permissions issues as well.

The various configuration options and their default values are specified in the documentation for this repository. Note the default location for logs to be processed in the Couchbase Fluent Bit 1.0.1 version of the image is slightly different to that used by Couchbase Server 6.6.2 so we override it above to use the same location on both. Later versions of the Couchbase Fluent Bit image align with the Couchbase Server location.

The container should now be running and processing logs from the directory we have locally to then send to its standard output stream by default. We can see this by a call to `docker logs logger` which should show logs being output as they update:

```
[0] couchbase.log.xdcr: [1622801916.603000000, {"filename"=>"/opt/couchbase/var/lib/couchbase/logs//goxdcr.log", "timestamp"=>"2021-06-04T10:18:36.603Z", "level"=>"INFO", "message"=>" GOXDCR.ResourceMgr: Resource Manager State = overallTP: 0 highTP: 0 highExist: false lowExist: false backlogExist: false maxTP: 0 highTPNeeded: 0 highTokens: 0 maxTokens: 0 lowTPLimit: 0 calibration: None dcpAction: Reset processCpu: 1 idleCpu: 95", "pod"=>"c901775dec2b", "logshipper"=>"couchbase.sidecar.fluentbit"}]
[0] couchbase.log.debug: [1622801986.855000000, {"filename"=>"/opt/couchbase/var/lib/couchbase/logs//debug.log", "logger"=>"ns_server", "level"=>"debug", "timestamp"=>"2021-06-04T10:19:46.855Z", "message"=>"ns_1@cb.local:compaction_daemon<0.532.0>:compaction_daemon:process_scheduler_message:1306]No buckets to compact for compact_views. Rescheduling compaction.", "pod"=>"c901775dec2b", "logshipper"=>"couchbase.sidecar.fluentbit"}]
[0] couchbase.log.http_access_internal: [1622801986.000000000, {"filename"=>"/opt/couchbase/var/lib/couchbase/logs//http_access_internal.log", "host"=>"127.0.0.1", "user"=>"@goxdcr-cbauth", "timestamp"=>"04/Jun/2021:10:19:46 +0000", "method"=>"GET", "path"=>"/pools/nodes", "code"=>"404", "size"=>"14", "client"=>"couchbase-goxdcr/6.6.2", "pod"=>"c901775dec2b", "logshipper"=>"couchbase.sidecar.fluentbit"}]
```

This is just an example of the output you may have. Note that each log file has its own stream using a Fluent Bit tag in the format: `couchbase.log.<name>`. This allows you to perform different processing or routing of individual logs, for example audit logs may need to go to a specific end point different from the rest or you may want to filter out lines from a particular log. You can even match multiple entries or the same entry to different outputs.

Running the container can be done as part of a startup script with systemd or similar as Couchbase Server would be. The container runtime can also automatically [start](https://docs.docker.com/engine/reference/run/#restart-policies---restart) specified containers every time.

## Loki and Grafana

The default configuration provided by the container is to send all the logs to standard output. However you can provide your own configuration file to use to do different things with no other change - to modify log processing and forwarding is just a configuration change which is one of the key benefits of fluent bit.

To highlight this, we are going to run up a local version of the Loki stack which is Grafana plus Loki for log capture (and Prometheus for metric capture). We will then configure our container to send logs to Loki so we can view them in Grafana graphically. The Loki and Grafana documentation has alternative ways to deploy it as well so refer to that for full details.

Make sure to stop our previously configured log forwarding container - you could also run up another just make sure to give it a different name: `docker stop logger`.

Run up Loki and Grafana now, making sure to expose the ports required for each (3100 and 3000 respectively):

```
docker run --rm -d --name loki -p 3100:3100 grafana/loki:2.0.0 -config.file=/etc/loki/local-config.yaml
docker run --rm -d --name grafana -p 3000:3000 -e GF_SECURITY_ADMIN_PASSWORD=password grafana/grafana:7.5.2
```

You can check both have started correctly with a call to `docker ps` and see their logs with a call to `docker logs <name>`.

To support forwarding to Loki we are going to get the IP address of the local container that is running it. Normally you would deploy it to a resolvable host or service name but for the demo we are running all as local containers. For the Docker Compose example in this directory, it automatically configures a host name we can use as part of the configuration which simplifies things locally. However to be complete, we are going to show you how to get the information you need.

To get just the IP address we can format the query as per the [official documentation](https://docs.docker.com/engine/reference/commandline/inspect/#get-an-instances-ip-address):

```
docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' loki
172.17.0.4
```

The example provided with Docker Compose here in the repository automatically provisions Grafana as well to use Loki but we need to do this manually here. If you connect to http://localhost:3000/login then you can log into Grafana as the `admin` user with the `password` in the environment variable above in the Grafana container run command.

We now need to add Loki as a datasource at: http://localhost:3000/datasources. Set up the data source using the IP address of the Loki container and port 3100 as forwarded when we ran the container.

### Custom configuration for log forwarding

Now we have Loki and Grafana configured correctly, let us create a new configuration for our log forwarder to send the logs to it. As part of our Couchbase Fluent Bit deployment we have broken up various sections of the configuration into reusable files to include so we can just use a two line [file](fluent-bit.conf) for this:

```
@include /fluent-bit/etc/fluent-bit.conf
@include /fluent-bit/etc/couchbase/out-loki.conf
```

This is what the example in this directory uses as we get Docker Compose to handle making the Loki container a resolvable host from the others.

For manually run containers, use the specific IP address we got earlier of the container running Loki in the configuration file (or a resolvable host name if on-premise, etc):

```
cat > /tmp/fluent-bit.conf << __EOF__
@include /fluent-bit/etc/fluent-bit.conf

[OUTPUT]
    name   loki
    match  *
    host 172.17.0.4
    labels job=couchbase-fluentbit
    label_keys $filename,$level

__EOF__
```

Now we can run up the log forwarding again but using this custom configuration, make sure you either rename it or stop the previous one:

```
docker run --rm -d --name logger -v /tmp/couchbase-logs/:/opt/couchbase/var/lib/couchbase/logs/:ro -e COUCHBASE_LOGS=/opt/couchbase/var/lib/couchbase/logs/ -v /tmp/fluent-bit.conf:/fluent-bit/config/fluent-bit.conf:ro couchbase/fluent-bit:1.0.1
```

Notice we also run everything as a read-only mounted filesystem so we cannot modify anything in the container - the raw logs are not touched.

### Viewing logs

Now logs should start being sent to Loki and Grafana can then view them: http://localhost:3000/explore?orgId=1&left=%5B%22now-1h%22,%22now%22,%22Loki%22,%7B%22expr%22:%22%7Bjob%3D%5C%22couchbase-fluentbit%5C%22%7D%22%7D%5D

You can now create dashboards and view the logs directly live in Grafana. Hopefully this gives you a nice taster for how you can use log forwarding with Couchbase Server and Fluent Bit.