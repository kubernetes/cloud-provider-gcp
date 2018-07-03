#!/bin/bash
set -o xtrace
set -o errexit
set -o nounset
set -o pipefail

if [ ! -d "vendor" ]; then
    echo must run from repo root
    exit 1
fi

# update deps
dep ensure
# restore BUILD files in vendor/
bazel run //:gazelle
# remove symlinks in vendor/
find vendor/ -type l | xargs rm
