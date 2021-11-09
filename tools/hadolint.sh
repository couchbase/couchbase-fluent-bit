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
set -eu
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
# Ensure we always have latest version
docker pull hadolint/hadolint

# Find all Dockerfiles assuming a certain naming convention
exit_code=0
while IFS= read -r -d '' file; do
    echo "Hadolint: .${file##"$SCRIPT_DIR/.."}"
    if ! docker run --rm -i hadolint/hadolint < "$file"; then
        exit_code=1
    fi
done < <(find "${SCRIPT_DIR}/.." -type d -path "*/go" -prune -o -type f -name '*dockerignore' -prune -o -type f -name 'Dockerfile*' -print0)

exit $exit_code
