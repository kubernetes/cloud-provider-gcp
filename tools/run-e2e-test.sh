#!/usr/bin/env bash

# Copyright 2026 The Kubernetes Authors.
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
#  - Starts a cluster with the "kops" tool via Makefile.
#  - Runs the e2e test binary against the cluster using the "ginkgo" tester.

set -o errexit
set -o pipefail
set -o xtrace

# Find the top-level directory.
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "${REPO_ROOT}"

# 1. Project Lifecycle Management (Boskos)
if [[ -z "${GCP_PROJECT:-}" ]]; then
    echo "GCP_PROJECT not set, acquiring project from boskos"
    source test/boskos.sh
    acquire_project
    export GCP_PROJECT="${PROJECT}"
    CLEANUP_BOSKOS="true"
fi

# 2. Cluster Lifecycle Management
export KOPS_CLUSTER_NAME="${CLUSTER_NAME:-run-e2e-test.k8s.local}"
make kops-up

# Define cleanup function
function cleanup {
  local exit_status=$?
  if [[ "${DELETE_CLUSTER:-true}" == "true" ]]; then
    make kops-down || echo "Warning: kops-down failed"
  fi
  if [[ "${CLEANUP_BOSKOS:-}" == "true" ]]; then
    source test/boskos.sh
    cleanup_boskos || echo "Warning: cleanup_boskos failed"
  fi
  exit "${exit_status}"
}
trap cleanup EXIT

# 3. Test Setup
export K8S_VERSION=$(make print-k8s-version)
export KOPS_BIN="${REPO_ROOT}/bin/kops"
export SSH_PRIVATE_KEY="${SSH_PRIVATE_KEY:-$(pwd)/../clusters/${KOPS_CLUSTER_NAME}/google_compute_engine}"
export KUBE_SSH_USER="${KUBE_SSH_USER:-${USER}}"
export GCP_LOCATION="${GCP_LOCATION:-us-central1}"
export GCP_ZONES="${GCP_ZONES:-${GCP_LOCATION}-b}"

# run-id is set to milliseconds since the unix epoch.
RUN_ID="$(date +%s%N | cut -b1-13)"
RUN_DIR="${REPO_ROOT}/_rundir/${RUN_ID}"
LOCAL_BIN="${REPO_ROOT}/bin"

# Ensure local bin is in PATH
export PATH="${LOCAL_BIN}:${PATH}"

test_args="--provider=gce --gce-project=${GCP_PROJECT} --gce-region=${GCP_LOCATION} --gce-zones=${GCP_ZONES} --minStartupPods=8"

# Create the run directory if it does not exist.
if [ ! -d "${RUN_DIR}" ]; then
    echo "Creating run dir: ${RUN_DIR}"
    mkdir -p "${RUN_DIR}"
fi

echo "Installing ginkgo binary"
GOBIN="${LOCAL_BIN}" go install github.com/onsi/ginkgo/v2/ginkgo@latest
cp "${LOCAL_BIN}/ginkgo" "${RUN_DIR}/ginkgo"

echo "Downloading installing kubectl (${K8S_VERSION})"
curl -LO -s "https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/amd64/kubectl"
chmod u+x ./kubectl
mv -f ./kubectl "${RUN_DIR}/kubectl"
echo

# Build the e2e.test binary and position it correctly.
echo "Build the e2e.test binary"
pushd "${REPO_ROOT}/test/e2e"
go test -c
popd
echo "Move e2e.test binary to: ${RUN_DIR}"
mv -f "${REPO_ROOT}/test/e2e/e2e.test" "${RUN_DIR}/e2e.test"

# Workaround kubetest2-kops/kops ambiguity by unsetting cluster name env vars
_CLUSTER_NAME="${KOPS_CLUSTER_NAME}"
unset KOPS_CLUSTER_NAME
unset CLUSTER_NAME

# Run the e2e test using the "kops" provider and the "ginkgo" tester.
echo "Running e2e test with k8s version: ${K8S_VERSION}"
kubetest2 kops \
  -v=2 \
  --cloud-provider=gce \
  --cluster-name="${_CLUSTER_NAME}" \
  --kops-binary-path="${KOPS_BIN}" \
  --ssh-private-key="${SSH_PRIVATE_KEY}" \
  --ssh-user="${KUBE_SSH_USER}" \
  --gcp-project="${GCP_PROJECT}" \
  --admin-access="${ADMIN_ACCESS:-0.0.0.0/0}" \
  --env="KOPS_FEATURE_FLAGS=${KOPS_FEATURE_FLAGS:-}" \
  --test=ginkgo \
  --kubernetes-version="${K8S_VERSION}" \
  --run-id=${RUN_ID} \
  -- \
  --use-built-binaries=true \
  --test-args="${test_args}" \
  --parallel=30 \
  --focus-regex='\[cloud-provider-gcp-e2e\]'

# 4. Teardown
if [[ "${DELETE_CLUSTER:-true}" == "true" ]]; then
  make kops-down
  DELETE_CLUSTER=false # Don't delete again in trap
fi

echo
echo "FINISHED Running cloud-provider-gcp e2e tests"
