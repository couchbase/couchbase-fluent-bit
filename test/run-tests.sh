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

# A simple test script to run various sanity checks on log processing. If these pass then we also
# run a golden diff of actual vs expected results.
# This is intended to run in the Fluent Bit debug container so has to use features supported by Busybox.
# All configuration is via environment variable with the intention to provide these to the container at runtime.
#
# The intention here is to allow this to be configurable so we can repeat with different logs.
# The only thing required is to specify the $COUCHBASE_LOGS and $COUCHBASE_AUDIT_LOGS directory with the expected logs in.
# For every log there should be an equivalent expected output with the same name and a .expected suffix.
# For example, debug.log would be run which produces a debug.log.actual and is then compared against debug.log.expected.
#
# docker run --rm -it --mount type=bind,source=$PWD/fluent-bit/test/logs/,target=/fluent-bit/test/logs/ couchbase/couchbase-fluent-bit-test
#
# Rebalance reports can also be tested, these must be in the usual $COUCHBASE_LOGS/rebalance sub-directory.
# If that directory is not present then rebalance checks are skipped.
set -uo pipefail

# Simple storage of any failures
exitCode=0

# Time in seconds before we end a test case verifying the log output.
# Required as exit_on_eof causes other problems: https://github.com/fluent/fluent-bit/issues/3274
# Tweak as required for running a complete test (depends on size of log, speed of host, etc.)
EXPECT_TEST_TIMEOUT=${EXPECT_TEST_TIMEOUT:-10}

# Do not run all unit tests by default, assumption is to do this on version change only.
# They are not 100% reliable so seem to have failures when run all together and/or timing issues.
# Set to yes in order to run them automatically.
RUN_FLUENT_BIT_TESTS=${RUN_FLUENT_BIT_TESTS:-no}

# Time in seconds before we end a fluent bit test case, mainly to prevent build stalling.
FLUENT_BIT_TEST_TIMEOUT=${FLUENT_BIT_TEST_TIMEOUT:-600}

export POD_NAMESPACE=default
export couchbase_node=cb-node
# Document settings and echo them together so we can see them in output
echo "COUCHBASE_LOGS_BINARY set to ${COUCHBASE_LOGS_BINARY} - using as fluent bit binary"
echo "COUCHBASE_LOGS set to ${COUCHBASE_LOGS} - put all log files here and expected output"
echo "COUCHBASE_LOGS_REBALANCE_TMP_DIR set to ${COUCHBASE_LOGS_REBALANCE_TMP_DIR} - temporary rebalance report output directory"
echo "EXPECT_TEST_TIMEOUT set to ${EXPECT_TEST_TIMEOUT} - the time taken to run each CI test before we kill it"
echo "FLUENT_BIT_TEST_TIMEOUT set to ${FLUENT_BIT_TEST_TIMEOUT} - the time taken to run each Fluent Bit test before we kill it"
echo "FLUENTBIT_VERSION set to ${FLUENTBIT_VERSION} - the version of fluent bit used for this container"
echo "COUCHBASE_FLUENTBIT_VERSION set to ${COUCHBASE_FLUENTBIT_VERSION} - the version of this container"

# Helper method to run the Fluent Bit expect tests and check for failures.
function runExpectTest() {
    testConfig=$1
    testLog="$2"

    # We have to timeout the test case as ending on EOF pauses/breaks the rest of the pipeline once it exits
    # https://github.com/fluent/fluent-bit/issues/3274
    # We need to use a KILL signal as that's the only one `timeout` supports on Busybox that actually works,
    # however this may generate some extra messages on other targets.
    timeout -s 9 "${EXPECT_TEST_TIMEOUT}" "${COUCHBASE_LOGS_BINARY}" --config "$testConfig" > "$testLog" 2>&1

    # Currently it always exits with 0 so we have to check for a specific error message.
    # https://github.com/fluent/fluent-bit/issues/3268
    if grep -iq -e "exception on rule" -e "invalid config" "$testLog" ; then
        cat "$testLog"
        cat "$testConfig"
        echo "FAILED: $testLog"
        exitCode=1
    else
        echo "PASSED: $testLog"
    fi
}

if [[ ! -x "${COUCHBASE_LOGS_BINARY}" ]]; then
    echo "FAILED: Unable to execute ${COUCHBASE_LOGS_BINARY}"
    exit 1
fi

# Run a sanity check that the supplied configurations are acceptable in their entirety
for i in /fluent-bit/etc/fluent*.conf; do
    # Ignore invalid/non-files
    [[ ! -f "$i" ]] && continue
    if [[ "$i" = "/fluent-bit/etc/fluent-bit-win32.conf" ]]; then
        continue
    fi
    if "${COUCHBASE_LOGS_BINARY}" --dry-run --config="$i"; then
        echo "PASSED: ${COUCHBASE_LOGS_BINARY} --dry-run --config=$i"
    else
        cat "$i"
        echo "FAILED: ${COUCHBASE_LOGS_BINARY} --dry-run --config=$i"
        exit 1
    fi
done

if [[ "${RUN_FLUENT_BIT_TESTS}" == "yes" ]]; then
    # Some of the tests need write permission
    cd /fluent-bit/test || (echo "FAILED: Unable to change directory for unit tests" && exit 1)
    # Run any additional binaries found, i.e. RHEL fluent bit unit tests
    for TEST in /fluent-bit/test/bin/*; do
        if [[ -x "${TEST}" ]]; then
            echo "Running ${TEST}"
            if  timeout -s 9 "${FLUENT_BIT_TEST_TIMEOUT}" "${TEST}" ; then
                echo "PASSED: $TEST"
            else
                echo "FAILED: $TEST"
                exitCode=1
            fi
        fi
    done
    if [[ $exitCode -ne 0 ]]; then
        echo "FAILED: Unit tests for fluent bit"
        exit $exitCode
    fi
fi

# Deal with any rebalance reports by invoking the watcher
if [[ -d "${COUCHBASE_LOGS}/rebalance" ]]; then
    # Test the removal of old files once we have >5 - create some dummy ones if we don't have an existing directory mounted in
    if [[ ! -d "${COUCHBASE_LOGS_REBALANCE_TMP_DIR}" ]]; then
        echo "Creating dummy files to test rotation of old files in rebalance processing"
        # Add a sub-directory to test that as well
        mkdir -p "${COUCHBASE_LOGS_REBALANCE_TMP_DIR}"/1
        # Does not work for busybox: `touch "${COUCHBASE_LOGS_REBALANCE_TMP_DIR}"/{2..10}.test`
        for i in $(seq 2 10); do
            touch "${COUCHBASE_LOGS_REBALANCE_TMP_DIR}/${i}.test"
            sleep 1 # Give us a slight modification time...
        done

        # Be careful not to mount extra ones in
        if [[ $(find "${COUCHBASE_LOGS_REBALANCE_TMP_DIR}" -maxdepth 1 -print | wc -l) -ne 11 ]]; then
            echo "FAILED: Unable to create files/directory to test in ${COUCHBASE_LOGS_REBALANCE_TMP_DIR}"
            ls -l "${COUCHBASE_LOGS_REBALANCE_TMP_DIR}"
            exitCode=1
        fi
    fi

    echo "Testing rebalance processing"
    # Run the watcher in the special mode to process existing and exit
    if /fluent-bit/bin/couchbase-watcher --ignoreExisting=false; then
        countOfInput=$(find "${COUCHBASE_LOGS}/rebalance" -type f -name "rebalance_report_*.json" -print |wc -l)
        countOfOutput=$(find "${COUCHBASE_LOGS_REBALANCE_TMP_DIR}" -type f -name "rebalance-processed-*.json" -print |wc -l)

        if [[ $countOfInput -eq $countOfOutput ]]; then
            echo "PASSED: Processed all rebalance reports"
            ls -l "${COUCHBASE_LOGS_REBALANCE_TMP_DIR}"
        else
            echo "FAILED: Unable to process rebalance reports, $countOfInput != $countOfOutput"
            exitCode=1
        fi
    else
        echo "FAILED: Unable to run rebalance processing"
        exitCode=1
    fi
else
    echo "SKIPPED: Rebalance report testing as no ${COUCHBASE_LOGS}/rebalance directory"
fi

# Make sure to wipe actuals otherwise we will just append
rm -f "${COUCHBASE_LOGS}"/*.log.actual

# Now run tests per input configuration so we can verify individually otherwise if any failed we would just exit with a failure.
for i in /fluent-bit/etc/couchbase/input/in-*.conf; do
    # Ignore invalid/non-files
    [[ ! -f "$i" ]] && continue
    # Skip rebalance if no reports
    [[ "$i" == "/fluent-bit/etc/couchbase/input/in-rebalance-report.conf" ]] && [[ ! -d "${COUCHBASE_LOGS_REBALANCE_TMP_DIR}" ]] && continue

    testConfig="$i.test-conf"
    testLog="$i.log"
    contents=$(sed $'/\[INPUT\]/a\    Read\_from\_head on' "$i")
    cat > "$testConfig" << __FB_EOF
@include /fluent-bit/test/conf/test-service.conf

# Now we include the configuration we want to test which should cover the logfile as well.
# We cannot exit when done as this then pauses the rest of the pipeline so leads to a race getting chunks out.
# https://github.com/fluent/fluent-bit/issues/3274
# Instead we rely on a timeout ending the test case.
$contents

@include /fluent-bit/test/conf/test-filters.conf
@include /fluent-bit/test/conf/test-output.conf
__FB_EOF
    runExpectTest "$testConfig" "$testLog"
done

# Run any other test cases we have locally (e.g. redaction)
for i in /fluent-bit/test/test-*.conf; do
    # Ignore invalid/non-files
    [[ ! -f "$i" ]] && continue
    testLog="$i.log"
    runExpectTest "$i" "$testLog"
done

# Run special test case
if bash /fluent-bit/test/run-file-transfer-tests.sh; then
    echo "PASSED: file transfer example script"
else
    echo "FAILED: file transfer example"
fi

# Now we run the golden diffs, i.e. compare actual to expected output
# We only work with any logs we have .expected output for
for i in "${COUCHBASE_LOGS}"/*.expected; do
    # Ignore invalid/non-files
    [[ ! -f "$i" ]] && continue

    expected=$i
    actual=${i%.expected}.actual
    if [[ ! -f "${actual}" ]]; then
        echo "FAILED: missing actual output $actual"
        exitCode=1
        continue
    fi

    # The FTS and Eventing logs have issues with timestamp parsing so we ignore those
    # Unfortunately Busybox is limited with tests/regexes so being very explicit here
    if [[ "$i" == "${COUCHBASE_LOGS}/fts.log.expected" || "$i" == "${COUCHBASE_LOGS}/eventing.log.expected" ]]; then
        echo "Ignoring timestamp deltas in $i"
        # Replace timestamps, e.g.: .actual: [1616875582.481815658, {"filename" --> .actual: [__IGNORED__, {"filename"
        sed -i 's/actual: \[.*\, {/actual: \[__IGNORED__, {/g' "$actual"
        sed -i 's/actual: \[.*\, {/actual: \[__IGNORED__, {/g' "$expected"
    fi

    if log-differ "${actual}" "${expected}"; then
        echo "PASSED: No differences found in $actual and $expected"
    elif diff -a -q "${actual}" "${expected}"; then
        echo "PASSED: No differences found in $actual and $expected"
    else
        echo "FAILED: Differences found between $actual and $expected"
        diff -a "${actual}" "${expected}"
        exitCode=1
    fi
done

# Finally confirm we have rebalance output although skip verifying the actual JSON
if [[ -d "${COUCHBASE_LOGS_REBALANCE_TMP_DIR}" ]]; then
    countOfInput=$(find "${COUCHBASE_LOGS}/rebalance" -type f -name "rebalance_report_*.json" -print |wc -l)
    countOfOutput=$(find "${COUCHBASE_LOGS_REBALANCE_TMP_DIR}" -type f -name "rebalance*.actual" -print |wc -l)

    if [[ $countOfInput -eq $countOfOutput ]]; then
        echo "PASSED: Handled all rebalance reports"
    else
        echo "FAILED: Unable to handle rebalance reports, $countOfInput != $countOfOutput"
        ls -l "${COUCHBASE_LOGS_REBALANCE_TMP_DIR}"
        exitCode=1
    fi
fi

exit $exitCode
