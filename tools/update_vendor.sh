#!/bin/bash
set -o xtrace
set -o errexit
set -o nounset
set -o pipefail

if [ ! -d "vendor" ]; then
    echo must run from repo root
    exit 1
fi

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# update vendor/
go mod vendor
# remove repo-originated BUILD files
find vendor -type f \( \
    -name BUILD \
    -o -name BUILD.bazel \
    -o -name '*.bzl' \
  \) -delete

# create a symlink in vendor directory pointing cloud-provider-gcp/providers to the //providers.
# This lets other packages and tools use the local staging components as if they were vendored.
rm -fr "${KUBE_ROOT}/vendor/k8s.io/cloud-provider-gcp/providers"
ln -s "../../../providers" "${KUBE_ROOT}/vendor/k8s.io/cloud-provider-gcp/providers"

# restore BUILD files in vendor/
bazel run //:gazelle
