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

# Monitor the loading of the container under test until it completes (or is stopped separately)
set -u
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

CONTAINER_UNDER_TEST=${CONTAINER_UNDER_TEST:-couchbase/fluent-bit-test:v1}
OUTPUT_FILE=${OUTPUT_FILE:-$SCRIPT_DIR/monitoring.csv}

CONTAINER=$(docker run -d -P --rm "${CONTAINER_UNDER_TEST}")
echo "Container running: ${CONTAINER}"
METRIC_PORT=$(docker inspect --format='{{(index (index .NetworkSettings.Ports "2020/tcp") 0).HostPort}}' "${CONTAINER}")

# Show logs and this will also exit when the container does
docker logs -f "${CONTAINER}" &

echo "Container,CPU Percentage,Memory Percentage,Memory Usage,Net IO,Block IO,PIDs" > "${OUTPUT_FILE}"
echo "Log output at each line" > "${OUTPUT_FILE}.log"
echo "Fluent bit metrics in JSON" > "${OUTPUT_FILE}.json"
while docker ps --no-trunc --filter "id=${CONTAINER}" | grep -q "${CONTAINER}"; do
    docker stats --all --no-stream --no-trunc --format "{{.Container}},{{.CPUPerc}},{{.MemPerc}},{{.MemUsage}},{{.NetIO}},{{.BlockIO}},{{.PIDs}}" "${CONTAINER}" >> "${OUTPUT_FILE}"
    docker logs --tail 1 "${CONTAINER}" >> "${OUTPUT_FILE}.log"
    curl --show-error --silent localhost:"${METRIC_PORT}"/api/v1/metrics >> "${OUTPUT_FILE}.json" 2>&1
    sleep 1
done
