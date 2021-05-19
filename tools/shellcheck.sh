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
# Find all shell scripts that are not part of the Go local directory used during build.
# Run Shellcheck on them.
# Pruning is a lot more performant as it does not descend into the directory.
# Note we cannot do an exec without some horrible mess to deal with the exit code collection.
# find "${SCRIPT_DIR}/../" \
#     -type d -path "*/go" -prune -o \
#     -type f \( -name '*.sh' -o -name '*.bash' \) -exec sh -c 'echo Shellcheck "$1"; docker run -i --rm koalaman/shellcheck:stable - < "$1"' sh {} \;
exitCode=0
while IFS= read -r -d '' file; do
    echo "Shellcheck: $file"
    if ! docker run -i --rm koalaman/shellcheck:stable - < "$file"; then
        exitCode=1
    fi
done < <(find "${SCRIPT_DIR}/.." -type d -path "*/go" -prune -o -type f \( -name '*.sh' -o -name '*.bash' \) -print0)

exit $exitCode
