#!/bin/bash
# A simple test script to run various sanity checks on log processing. If these pass then we also
# run a golden diff of actual vs expected results.
# This is intended to run in the Fluent Bit debug container so has to use features supported by Busybox.
# All configuration is via environment variable with the intention to provide these to the container at runtime.
#
# The intention here is to allow this to be configurable so we can repeat with different logs.
# The only thing required is to specify the $COUCHBASE_LOGS directory with the expected logs in.
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

# Time in seconds before we end a test case.
# Required as exit_on_eof causes other problems: https://github.com/fluent/fluent-bit/issues/3274
# Tweak as required for running a complete test (depends on size of log, speed of host, etc.)
TEST_TIMEOUT=${TEST_TIMEOUT:-10}

# Document settings and echo them together so we can see them in output
echo "COUCHBASE_LOGS set to ${COUCHBASE_LOGS} - put all log files here and expected output"
echo "COUCHBASE_LOGS_REBALANCE_TEMPDIR set to ${COUCHBASE_LOGS_REBALANCE_TEMPDIR} - temporary rebalance report output directory"
echo "TEST_TIMEOUT set to ${TEST_TIMEOUT} - the time taken to run each CI test before we kill it"

# Helper method to run the Fluent Bit expect tests and check for failures.
function runExpectTest() {
    testConfig=$1
    testLog="$2"

    # We have to timeout the test case as ending on EOF pauses/breaks the rest of the pipeline once it exits
    # https://github.com/fluent/fluent-bit/issues/3274
    timeout -s 9 "${TEST_TIMEOUT}" /fluent-bit/bin/fluent-bit --config "$testConfig" > "$testLog" 2>&1
    
    # Currently it always exits with 0 so we have to check for a specific error message.
    # https://github.com/fluent/fluent-bit/issues/3268
    if grep -iq "exception on rule" "$testLog" ; then 
        cat "$testLog"
        cat "$testConfig"
        echo "FAILED: $testLog" 
        exitCode=1
    else
        echo "PASSED: $testLog"
    fi
}

# Deal with any rebalance reports by invoking the watcher
if [[ -d "${COUCHBASE_LOGS}/rebalance" ]]; then 
    # Test the removal of old files once we have >5 - create some dummy ones if we don't have an existing directory mounted in
    if [[ ! -d "${COUCHBASE_LOGS_REBALANCE_TEMPDIR}" ]]; then
        # Add a sub-directory to test that as well
        mkdir -p "${COUCHBASE_LOGS_REBALANCE_TEMPDIR}"/1
        #touch "${COUCHBASE_LOGS_REBALANCE_TEMPDIR}"/{2..10}.test
        for i in $(seq 2 10); do
            touch "${COUCHBASE_LOGS_REBALANCE_TEMPDIR}/${i}.test"
            sleep 1 # Give us a slight modification time...
        done

        # Be careful not to mount extra ones in
        if [[ $(find "${COUCHBASE_LOGS_REBALANCE_TEMPDIR}" -maxdepth 1 -print0 | wc -l) -ne 10 ]]; then
            echo "FAILED: Unable to create files/directory to test in ${COUCHBASE_LOGS_REBALANCE_TEMPDIR}"
        fi
    fi

    # Run the watcher in the special mode to process existing and exit
    if /fluent-bit/bin/couchbase-watcher --ignoreExisting=false; then 
        countOfInput=$(find "${COUCHBASE_LOGS}/rebalance" -type f -name "rebalance_report_*.json" -print0 |wc -l)
        countOfOutput=$(find "${COUCHBASE_LOGS_REBALANCE_TEMPDIR}" -type f -name "rebalance-processed-*.json" -print0 |wc -l)

        if [[ $countOfInput -eq $countOfOutput ]]; then
            echo "PASSED: Processed all rebalance reports"
            ls -l "${COUCHBASE_LOGS_REBALANCE_TEMPDIR}"
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
for i in /fluent-bit/etc/couchbase/in-*.conf; do
    # Ignore invalid/non-files
    [[ ! -f "$i" ]] && continue
    # Skip rebalance if no reports
    [[ "$i" == "/fluent-bit/etc/couchbase/in-rebalance-report.conf" ]] && [[ ! -d "${COUCHBASE_LOGS_REBALANCE_TEMPDIR}" ]] && continue

    testConfig="$i.test-conf"
    testLog="$i.log"
    cat > "$testConfig" << __FB_EOF
@include /fluent-bit/test/conf/test-service.conf

# Now we include the configuration we want to test which should cover the logfile as well.
# We cannot exit when done as this then pauses the rest of the pipeline so leads to a race getting chunks out.
# https://github.com/fluent/fluent-bit/issues/3274
# Instead we rely on a timeout ending the test case.
@include $i
    Read_from_head On

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

# Now we run the golden diffs, i.e. compare actual to expected output
if [[ $exitCode -eq 0 ]]; then
    # We only work with any logs we have .expected output for
    for i in "${COUCHBASE_LOGS}"/*.log.expected; do
        # Ignore invalid/non-files
        [[ ! -f "$i" ]] && continue

        expected=$i
        actual=${i%.expected}.actual
        if [[ ! -f "${actual}" ]]; then
            echo "FAILED: missing actual output $actual"
            exitCode=1
            continue
        fi

        if diff -a -q "${actual}" "${expected}"; then
            echo "FAILED: Differences found between $actual and $expected"
            diff -a "${actual}" "${expected}"
            exitCode=1
        else
            echo "PASSED: No differences found in $actual and $expected"
        fi
    done
else 
    echo "SKIPPED: Golden diffs as previous failures"
fi

exit $exitCode