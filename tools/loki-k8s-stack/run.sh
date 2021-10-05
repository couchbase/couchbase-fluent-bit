#!/bin/bash
# Copyright 2021 Couchbase, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file  except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the  License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# A simple script to run the compose stack.
# This will run up a server image with an attached logging image that has a slightly customised
# configuration which will exit if any issues found with the output.
# It uses the Fluent Bit EXPECT filter to do this.
set -eu

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Provided during build
DOCKER_USER=${DOCKER_USER:-couchbase}
DOCKER_TAG=${DOCKER_TAG:-v1}

# The time between checks that the containers are running
TEST_PERIOD_SECONDS=${TEST_PERIOD_SECONDS:-10}
# The number of checks to do before exiting (will exit sooner if check fails)
MAX_ITERATIONS=${MAX_ITERATIONS:-60}
# The server image under test
SERVER_IMAGE=${SERVER_IMAGE:-couchbase/server:7.0.1}
# The fluent bit image under test
FLUENT_BIT_IMAGE=${FLUENT_BIT_IMAGE:-$DOCKER_USER/fluent-bit:$DOCKER_TAG}

# In case you want a different name
CLUSTER_NAME=${CLUSTER_NAME:-couchbase-fluentbit-loki}
# The server container image to use
SERVER_IMAGE=${SERVER_IMAGE:-couchbase/server:7.0.1}
SERVER_COUNT=${SERVER_COUNT:-3}

# Build the container
make -C "${SCRIPT_DIR}/../.." container

# Delete the old cluster (if it exists)
kind delete cluster --name="${CLUSTER_NAME}"

# Create KIND cluster with 1 worker node
# Mostly just an example to show you how to do it
kind create cluster --name="${CLUSTER_NAME}" --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
- role: worker
EOF

# Speed up deployment by pre-loading the server image
docker pull "${SERVER_IMAGE}"
kind load docker-image "${SERVER_IMAGE}" --name="${CLUSTER_NAME}"
kind load docker-image "${FLUENT_BIT_IMAGE}" --name="${CLUSTER_NAME}"

# Set up helm repos
helm repo add grafana https://grafana.github.io/helm-charts || helm repo add grafana https://grafana.github.io/helm-charts/
helm repo add couchbase https://couchbase-partners.github.io/helm-charts/ || helm repo add couchbase https://couchbase-partners.github.io/helm-charts
helm repo update

# Add Loki stack via helm chart in a separate namespace
helm upgrade --install loki --namespace=monitoring --create-namespace grafana/loki-stack \
  --set fluent-bit.enabled=false,promtail.enabled=false,grafana.enabled=true,prometheus.enabled=false

# Add Couchbase via helm chart: note with/out slash is considered different so we just try both
helm upgrade --install couchbase --namespace=couchbase --create-namespace couchbase/couchbase-operator \
    --set cluster.image="${SERVER_IMAGE}",cluster.servers.default.size="${SERVER_COUNT}",cluster.logging.server.sidecar.image="${FLUENT_BIT_IMAGE}" \
    --values="${SCRIPT_DIR}/values.yaml"
# To tweak the helm deployment there are a lots of options.
# All the configuration values are here: https://github.com/couchbase-partners/helm-charts/blob/master/charts/couchbase-operator/values.yaml
# For more details refer to the official documentation: https://docs.couchbase.com/operator/current/helm-setup-guide.html

# Wait for deployment to complete, the --wait flag does not work for this.
echo "Waiting for CB to start up..."
# The operator uses readiness gates to hold the containers until the cluster is actually ready to be used
until [[ $(kubectl get pods --namespace=couchbase --field-selector=status.phase=Running --selector='app=couchbase' --no-headers 2>/dev/null |wc -l) -eq $SERVER_COUNT ]]; do
    echo -n '.'
    sleep 2
done
echo "CB configured and ready to go"

kubectl get secret --namespace monitoring loki-grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo
kubectl port-forward --namespace monitoring service/loki-grafana 3000:80