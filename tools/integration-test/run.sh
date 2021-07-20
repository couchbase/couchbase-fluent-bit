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
SERVER_IMAGE=${SERVER_IMAGE:-couchbase/server:6.6.2}
# The fluent bit image under test
FLUENT_BIT_IMAGE=${FLUENT_BIT_IMAGE:-$DOCKER_USER/fluent-bit:$DOCKER_TAG}
# Ensure we run the correct compose file from the correct location regardless of current directory
pushd "${SCRIPT_DIR}"

# Clean up then start new instances
docker-compose rm -v --force --stop
docker-compose up -d --force-recreate --remove-orphans

# Check every $TEST_PERIOD_SECONDS that we are still running for a maximum of $MAX_ITERATIONS
COUNTER=0
while [[ $COUNTER -lt $MAX_ITERATIONS ]]; do
    echo "Iteration $COUNTER of $MAX_ITERATIONS"
    (( COUNTER=COUNTER+1 ))
    # Ensure that our containers have not exited
    if [[ $(docker-compose ps --services --filter "status=running" | wc -l) -ne 2 ]]; then
        echo "FAILED: a container has exited"
        docker-compose ps
        # Output any useful debug information
        docker-compose logs
        # Grab all logs to help with regression tests in the future
        tar -czvf "${SCRIPT_DIR}/server-logs.tar.gz" -C "${SCRIPT_DIR}/logs/" .
        # Cleanup
        docker-compose rm -v --force --stop
        popd
        exit 1
    fi
    sleep "$TEST_PERIOD_SECONDS"
done

docker-compose rm -v --force --stop
echo "SUCCESS: all containers still running"
popd