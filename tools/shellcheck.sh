#!/bin/bash
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
