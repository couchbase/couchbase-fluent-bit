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

# Simple file transfer example line-by-line
set -ux
FILE_TRANSFER_TEST_TIMEOUT=${FILE_TRANSFER_TEST_TIMEOUT:-10}

# Run up the receiver first
timeout -s 9 "${FILE_TRANSFER_TEST_TIMEOUT}" "${COUCHBASE_LOGS_BINARY}" --config "/fluent-bit/test/conf/file-transfer/test-file-transfer-dest.conf" 2>&1 &

# Now, run up the sender
timeout -s 9 "${FILE_TRANSFER_TEST_TIMEOUT}" "${COUCHBASE_LOGS_BINARY}" --config "/fluent-bit/test/conf/file-transfer/test-file-transfer-source.conf" 2>&1

# They should output a file we can compare later but you can also review in the log what has been sent - only the receiver outputs to stdout
wait -n
