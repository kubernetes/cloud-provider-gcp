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
find vendor -type f \( \
    -name BUILD \
    -o -name BUILD.bazel \
    -o -name '*.bzl' \
  \) -delete
# restore BUILD files in vendor/
bazel run //:gazelle
