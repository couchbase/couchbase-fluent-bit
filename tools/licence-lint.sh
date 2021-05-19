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

# Simple script to check all files have the appropriate copyright, will fail and list them if not.
set -eu
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

exitCode=0
while IFS= read -r -d '' SOURCE
do
    if ! head "${SOURCE}" | grep -q Copyright; then
        echo ".${SOURCE##$SCRIPT_DIR/..}: Missing copyright"
        exitCode=1
    fi
    if ! head "${SOURCE}" | grep -q 'Apache License, Version 2.0'; then
        echo ".${SOURCE##$SCRIPT_DIR/..}: Missing licence"
        exitCode=1
    fi
done < <(find "${SCRIPT_DIR}/.." -type d -path "*/go" -prune -o -type f \( -name '*.go' -o -name '*.sh' \) -print0)
# Make sure we prune out any local Go installation directory

exit $exitCode