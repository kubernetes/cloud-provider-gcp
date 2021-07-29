#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

CLOUD_PROVIDER_LKG_FILE="$(git rev-parse --show-toplevel)/PROVIDER_LKG_INFO"
CLOUD_PROVIDER_LKG_HASH=$(git rev-parse --short HEAD)
echo $CLOUD_PROVIDER_LKG_HASH > $CLOUD_PROVIDER_LKG_FILE
git add $CLOUD_PROVIDER_LKG_FILE
git commit -m "Update cloud-provider-gcp LKG version to $CLOUD_PROVIDER_LKG_HASH"
