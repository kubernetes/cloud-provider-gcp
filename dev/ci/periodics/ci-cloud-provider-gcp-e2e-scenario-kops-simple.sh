#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

cd $GOPATH/src/cloud-provider-gcp
e2e/add-kubernetes-to-workspace.sh
e2e/scenarios/kops-simple
