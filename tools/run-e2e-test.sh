#!/usr/bin/env bash

# Copyright 2024 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Runs the e2e tests.
#  Can be run in two modes: 1) command-line, and 2) prow CI.
#  - Builds the e2e test binary (i.e. "test/e2e/e2e.test").
#  - Packages e2e test binary with "ginkgo" and "kubectl" binaries.
#  - Starts a cluster with kubetest2, using the "kops" deployer.
#    - Cluster started in either passed cluster (GCP_CLUSTER env var),
#      or "boskos" cluster in prow CI.
#    - Cluster version is latest stable version.
#  - Runs the e2e test binary against the cluster using the "ginkgo" tester.

# Parameter:
#   GCP_PROJECT - if running from command line, this env var needs
#     to be set to a project the user has credentials for.
#   GCP_ZONE - an optional env var (e.g. us-central1-c) for zone.
#   Requires both regular and application default credentials if
#     running from the command line.
#     - gcloud auth login
#     - glcoud auth application-default login
#

set -o errexit
set -o pipefail

# Find the top-level directory.
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "${REPO_ROOT}"

# Import the boskos code.
source ${REPO_ROOT}/test/boskos.sh

# Source kops test library
source "${REPO_ROOT}/test/kops.sh"

# Setup environment
kops_setup_env

# # Build and push images
# kops_build_and_push_images

# Setup cleanup trap
if [[ -z "${DELETE_CLUSTER:-}" ]]; then
  DELETE_CLUSTER="true"
fi
export DELETE_CLUSTER
trap kops_cleanup EXIT

# Create cluster
kops_create_cluster

# Parameters
#
VERBOSITY=2
# run-id is set to milliseconds since the unix epoch.
RUN_ID="$(date +%s%N | cut -b1-13)"
RUN_DIR="${REPO_ROOT}/_rundir/${RUN_ID}"
# Ensure GOPATH/GOBIN are set.
if [[ ! -v GOPATH ]]; then
    GOPATH="${HOME}/go"
fi
if [[ ! -v GOBIN ]]; then
    GOBIN="${GOPATH}/bin"
fi

test_args="--provider=gce --gce-project=${GCP_PROJECT} --gce-region=${GCP_LOCATION} --gce-zones=${GCP_ZONES} --minStartupPods=8"

# This script is currently using the "use-built-binaries" option, which
# requires the built "e2e.test" binary as well as the "kubectl" and "ginkgo"
# binaries to exist in the "_rundir/${RUN_ID}" directory.

# Create the run directory if it does not exist.
#
if [ ! -d "${RUN_DIR}" ]; then
    echo "Creating run dir: ${RUN_DIR}"
    mkdir -p "${RUN_DIR}"
fi

echo "Downloading/installing kubetest2"
go install sigs.k8s.io/kubetest2@latest

echo "Downloading/installing kubetest2 ginkgo tester"
go install sigs.k8s.io/kubetest2/kubetest2-tester-ginkgo@latest

echo "Downloading/installing ginkgo binary"
go install github.com/onsi/ginkgo/v2/ginkgo@latest
cp "${GOBIN}/ginkgo" "${RUN_DIR}/ginkgo"

echo "Downloading installing kubectl (${K8S_VERSION})"
curl -LO -s "https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/amd64/kubectl"
chmod u+x ./kubectl
mv -f ./kubectl "${RUN_DIR}/kubectl"
echo

# Build the e2e.test binary and position it correctly.
#
echo "Build the e2e.test binary"
pushd "${REPO_ROOT}/test/e2e"
go test -c
popd
echo "Move e2e.test binary to: ${RUN_DIR}"
mv -f "${REPO_ROOT}/test/e2e/e2e.test" "${RUN_DIR}/e2e.test"

# Run the e2e test using the "kops" provider and the "ginkgo" tester.
echo "Running e2e test with k8s version: ${K8S_VERSION}"
kubetest2 kops ${KUBETEST2_ARGS} \
  --test=ginkgo \
  --kubernetes-version="${K8S_VERSION}" \
  --run-id=${RUN_ID} \
  -- \
  --use-built-binaries=true \
  --test-args="${test_args}" \
  --parallel=30 \
  --focus-regex='\[cloud-provider-gcp-e2e\]'

# Delete cluster
if [[ "${DELETE_CLUSTER:-}" == "true" ]]; then
  kops_delete_cluster
  DELETE_CLUSTER=false # Don't delete again in trap
fi

echo
echo "FINISHED Running cloud-provider-gcp e2e tests"
