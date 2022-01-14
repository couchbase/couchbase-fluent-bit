# Splunk with Couchbase Server

This directory includes everything required to run a simple stack locally using [Docker Compose](https://docs.docker.com/compose/) that supports log forwarding from a Couchbase Server instance to a single instance of [Splunk Enterprise](https://hub.docker.com/r/splunk/splunk) via the [HTTP Event Collector (HEC)](https://docs.splunk.com/Documentation/SplunkCloud/8.2.2111/Data/UsetheHTTPEventCollector).

## Pre-requisites
* Docker: https://docs.docker.com/engine/install/
* Docker Compose: https://docs.docker.com/compose/install/

## Usage

The Couchbase Server container is purely run as an example to generate the logs.

First change `SPLUNK_TOKEN` and `SPLUNK_PASSWORD` to something more secure in the .env file.

Then, simply run `docker-compose up` from this directory to start everything up.

To change any image versions this can be specified by the environment variables referenced in the Docker Compose [file](docker-compose.yml) with defaults in the environment [file](.env).

## Architecture

The whole example covers:
* Couchbase Server with bind mount for log directory - this is a preconfigured instance by default.
* Couchbase Fluent Bit with read-only bind mount for log directory and custom configuration to send to Splunk.
* Splunk configured with HTTP Event Collector (HEC) to receive the logs.

## Splunk and HTTP Event Collector (HEC)

Splunk reads its default values in from `default.yml` however any environment variables take precedence.

Please see [Create standalone with HEC](https://splunk.github.io/docker-splunk/EXAMPLES.html#create-standalone-with-hec) for more configuration options.

### Viewing logs

Now logs should start being sent to the HEC and viewable in Splunk. By visiting `http://localhost:8000/en-GB/app/search/search?q=search%20source%3D%22http%3Asplunk_hec_token%22` you can begin to start exploring the events.
