#!/bin/bash
# Simple script to take a Fluent Bit config file with @include statements and "flatten" it into a single file via stdout.
# Removes blank lines or those with comments as well.
# Primary use case is for the visualiser support: https://config.calyptia.com/
set -eu
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# The runtime location we want to replace
REPLACE_DIR=${REPLACE_DIR:-/fluent-bit/etc}
# The repo location to substitute the runtime location with
CONFIG_DIR=${CONFIG_DIR:-$SCRIPT_DIR/../conf}
# The file we want to start with
CONFIG_FILE=${CONFIG_FILE:-$CONFIG_DIR/fluent-bit.conf}

function readFile() {
    FILE=$1
    while IFS="" read -r LINE || [[ -n "${LINE}" ]]
    do
        # Skip the comments
        [[ "${LINE}" =~ ^[[:blank:]]*#.*$ ]] && continue
        # Skip blank lines
        [[ "${LINE}" =~ ^[[:space:]]*$ ]] && continue

        # Now find our include lines
        if [[ ${LINE} == @include* ]]; then
            # Remove the @include prefix to get just the filename
            NESTED_INCLUDE=${LINE##@include }
            # Filenames are intended for runtime paths in the container so substitute with local repo paths
            ACTUAL_FILE=${NESTED_INCLUDE/$REPLACE_DIR/$CONFIG_DIR}
            # Include this file and any it then includes recursively
            readFile "${ACTUAL_FILE}"
        else
            echo "${LINE}"
        fi
    done < "${FILE}"
}

readFile "${CONFIG_FILE}"
