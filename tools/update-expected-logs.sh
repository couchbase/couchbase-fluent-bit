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

# Simple script to run the test container and generate new expected logs for future comparison.
set -u
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

TEST_CONTAINER=${TEST_CONTAINER:-couchbase/fluent-bit-test:v1}
OUTPUT_LOGS_DIR=${OUTPUT_LOGS_DIR:-${SCRIPT_DIR}/../test/logs}

rm -f "${OUTPUT_LOGS_DIR}/*.actual"
rm -rf "${OUTPUT_LOGS_DIR}/rebalance-logs/"
# This may fail but can be ignored
docker run --rm -v "${OUTPUT_LOGS_DIR}/":/fluent-bit/test/logs/ -e COUCHBASE_LOGS=/fluent-bit/test/logs "${TEST_CONTAINER}"

rm -rf "${OUTPUT_LOGS_DIR}/rebalance-logs/"

while IFS= read -r -d '' file; do
    destination=${file%%.actual}.expected
    echo "Updating: $file ==> $destination"
    mv -f "$file" "$destination"
done < <(find "${OUTPUT_LOGS_DIR}" -type f -name '*.actual' -print0)
