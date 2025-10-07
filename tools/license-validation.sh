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

set -euo pipefail

IMAGE="$1"

# Run a check to ensure each of the files under /licenses/ required by redhat certification exist and haven't changed.
exitCode=0

check_license_file() {
    local file="$1"
    local expected_hash="$2"
    
    # Compute hash inside container
    local actual_hash
    actual_hash=$(docker run --rm --entrypoint="" "$IMAGE" sh -c "sha256sum '$file' 2>/dev/null | awk '{print \$1}' || echo 'MISSING'")

    if [ "$actual_hash" = "MISSING" ]; then
        echo "Missing license file: $file"
        exitCode=1
    fi

    if [ "$actual_hash" != "$expected_hash" ]; then
        echo "License file invalid: $file"
        exitCode=1
    fi
}

# Check all license files
check_license_file "/licenses/couchbase.txt" "0cec06e0e55fbc3dc5cee4fca9b607f66cb8f4e4dbcf3b3c013594dd156732e9"
check_license_file "/licenses/fluent-bit.txt" "0cec06e0e55fbc3dc5cee4fca9b607f66cb8f4e4dbcf3b3c013594dd156732e9"
check_license_file "/licenses/kubesphere.txt" "0cec06e0e55fbc3dc5cee4fca9b607f66cb8f4e4dbcf3b3c013594dd156732e9"
check_license_file "/licenses/sha1.txt" "0885d2886db2284f9dcb39e02a6b9380eacc4dc404fa9e2fcc60335dd7ee7fa0"

if [ $exitCode -eq 0 ]; then
    echo "All license files validated successfully"
fi

exit $exitCode