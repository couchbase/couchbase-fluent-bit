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
set -u
FILE_TRANSFER_TEST_TIMEOUT=${FILE_TRANSFER_TEST_TIMEOUT:-10}

# Run up the receiver first
timeout -s 9 "${FILE_TRANSFER_TEST_TIMEOUT}" "${COUCHBASE_LOGS_BINARY}" --config "/fluent-bit/test/conf/file-transfer/test-file-transfer-dest.conf" 2>&1 &
RECEIVER_PID=$!

# Now, run up the sender
timeout -s 9 "${FILE_TRANSFER_TEST_TIMEOUT}" "${COUCHBASE_LOGS_BINARY}" --config "/fluent-bit/test/conf/file-transfer/test-file-transfer-source.conf" 2>&1
SENDER_RC=$?

sleep 2
# After a small grace, we check if the receiver is still running and kill it if it is.
# We then return the sender's exit code.
if kill -0 "$RECEIVER_PID" 2>/dev/null; then
  kill "$RECEIVER_PID" 2>/dev/null
  wait "$RECEIVER_PID" 2>/dev/null || true
fi

exit "$SENDER_RC"
