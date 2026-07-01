#!/bin/bash

# Update vendor dependencies for the OTE (openshift-tests) suite and commit
# the result as a carry patch.
#
# Intended to be run as a rebasebot post-rebase hook:
#   --post-rebase-hook git:main:/openshift-hack/update-ote.sh

set -e
set -o pipefail

OTE_DIR="${REBASEBOT_WORKING_DIR}/openshift-tests"

echo "Updating OTE vendor dependencies in ${OTE_DIR}"
pushd "${OTE_DIR}" > /dev/null

make vendor

popd > /dev/null

if [[ -z "$REBASEBOT_GIT_USERNAME" || -z "$REBASEBOT_GIT_EMAIL" ]]; then
    author_flag=()
else
    author_flag=(--author="${REBASEBOT_GIT_USERNAME} <${REBASEBOT_GIT_EMAIL}>")
fi

if [[ -n $(git status --porcelain openshift-tests/) ]]; then
    git add openshift-tests/
    git commit "${author_flag[@]}" \
        -m "UPSTREAM: <carry>: Update OTE vendor dependencies"
else
    echo "No changes to OTE vendor dependencies"
fi
