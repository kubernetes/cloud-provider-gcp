#!/bin/bash
set -o xtrace
set -o errexit
set -o nounset
set -o pipefail

if [ ! -d "vendor" ]; then
    echo must run from repo root
    exit 1
fi

# update vendor/
go mod vendor
# remove repo-originated BUILD files
find vendor/ -name BUILD | xargs rm
# restore BUILD files in vendor/
bazel run //:gazelle
