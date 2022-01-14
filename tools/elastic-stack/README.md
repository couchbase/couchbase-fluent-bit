# Elasticsearch with Couchbase Server

This directory includes everything required to run a simple stack locally using [Docker Compose](https://docs.docker.com/compose/) that supports log forwarding from a Couchbase Server instance to a multi node cluster of [Elasticsearch](https://www.elastic.co/elasticsearch/) and then visualising the logs in [Kibana](https://www.elastic.co/kibana/).

## Pre-requisites
* Docker: https://docs.docker.com/engine/install/
* Docker Compose: https://docs.docker.com/compose/install/

## Usage

The Couchbase Server container is purely run as an example to generate the logs.

Before running, it is most likely you will need to set `vm.max_map_count` to at least 262144. As per the [Install Elasticsearch with Docker](https://www.elastic.co/guide/en/elasticsearch/reference/current/docker.html#_set_vm_max_map_count_to_at_least_262144)

To run, simply run `docker-compose up` from this directory to start everything up.

The script brings up the stack in two parts, initially bringing up Couchbase Server, Elasticsearch and Kibana, then the Couchbase Fluent Bit Log. Between stages a curl command is run to create the proper timestamp mapping. This update can't be done once data is already streaming. See [Elasticsearch Quirks](#Elasticsearch-Quirks) for more details
To view the docker compose logs type `docker-compose logs` in this directory.

To change any image versions this can be specified by the environment variables referenced in the Docker Compose [file](docker-compose.yml) with defaults in the environment [file](.env).

## Architecture

The whole example covers:
* Couchbase Server with bind mount for log directory - this is a preconfigured instance by default.
* Couchbase Fluent Bit with read-only bind mount for log directory and custom configuration to send to Elasticsearch.
* Multi node Elastic Search to receive the logs.
* Kibana preconfigured to talk to Elasticsearch.


## Elasticsearch and Kibana
The setup of Kibana and Elasticsearch is entirely based on [their TLS example](https://github.com/elastic/stack-docs/blob/bfd6f7d201162cf6736883f9b2086f3e680e9a4d/docs/en/getting-started/docker/docker-compose.yml). 
Kibana is configured via environment variables to automatically connect to Elasticsearch.

```
ELASTICSEARCH_URL: http://elasticsearch:9200
ELASTICSEARCH_HOSTS: http://elasticsearch:9200
```
See [Running the Elastic Stack on Docker](https://www.elastic.co/guide/en/elastic-stack-get-started/current/get-started-docker.html) for more configuration options

### Viewing logs

Once Kibana is running you should be able to sign in with the username `elastic` and whatever password is set in the environment file.
Now logs should start being sent to Elasticsearch and viewable in Kibana. The created index should be viewable at `http://localhost:5601/app/kibana#/management/elasticsearch/index_management/indices` as `couchbase`

You can now create dashboards and explore the logs directly live in Kibana.
## Elasticsearch Quirks
The default parser for timestamp dates in Elastic Search doesn't support the format `dd/MMM/yyyy:HH:mm:ss Z`

This means that the index mapping has to be updated before data is ingested into the index.

The below PUT request extends the base `[strict_date_optional_time||epoch_milli]` options to include `dd/MMM/yyyy:HH:mm:ss Z`.

Once the command has been run, and the timestamp property format updated, then Couchbase Fluent Bit's Elasticsearch output can be enabled.

```
curl -X PUT "localhost:9200/couchbase?pretty" -H 'Content-Type: application/json' -d'
{
  "mappings": {
    "properties": {
      "timestamp": {
        "type":   "date",
        "format": "dd/MMM/yyyy:HH:mm:ss Z||strict_date_optional_time||epoch_millis"
      }
    }
  }
}
`
```
