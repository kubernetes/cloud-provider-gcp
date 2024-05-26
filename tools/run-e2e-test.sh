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
#  - Starts a cluster with kubetest2, using the "gce" deployer.
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

# Parameters
#
# The cluster will run the latest stable released k8s binaries.
VERSION=$(curl -L -s https://dl.k8s.io/release/stable.txt)
VERBOSITY=2
DEFAULT_GCP_ZONE="us-central1-b"
MASTER_SIZE="n1-standard-8"
NODE_SIZE="n1-standard-4"
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

# Trap interrupt, error, and exit to cleanup.
cleanup() {
    # The PROJECT env var is set when acquiring a boskos project.
    if [[ -v PROJECT ]]; then
	echo "Cleaning up boskos..."
	cleanup_boskos
    fi

    echo "Cleaning up run dir: ${RUN_DIR}"
    rm -rf ${RUN_DIR}
}
trap cleanup INT TERM EXIT

# Set up (for cli runs) or acquire a (in case of prow CI) GCP project
# for bringing up the cluster.
bazel_user=""
if [[ ! -v  GCP_PROJECT ]]; then
    # GCP_PROJECT not defined, so attempt to acquire a boskos project
    echo "No GCP Project specified...attempting to acquire boskos project"
    acquire_project
    GCP_PROJECT=${PROJECT}
    echo "Boskos Project: ${GCP_PROJECT}"
    if [[ ! -v GCP_ZONE ]]; then
	GCP_ZONE=${DEFAULT_GCP_ZONE}
    fi
    echo "GCP ZONE: ${GCP_ZONE}"
else
    echo "GCP Project specified: ${GCP_PROJECT}"
    if [[ ! -v GCP_ZONE ]]; then
	GCP_ZONE=${DEFAULT_GCP_ZONE}
    fi
    echo "GCP ZONE: ${GCP_ZONE}"
    # Install bazel in user directory if running from command-line.
    bazel_user="--user"
fi
gcp_project_args="--gcp-project ${GCP_PROJECT} --gcp-zone ${GCP_ZONE} "
test_args="--provider=gce --gce-project=${GCP_PROJECT} --gce-zone=${GCP_ZONE} --minStartupPods=8"

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

echo "Downloading/installing kubetest2 gce deployer"
go install sigs.k8s.io/kubetest2/kubetest2-gce@latest

echo "Downloading/installing kubetest2 ginkgo tester"
go install sigs.k8s.io/kubetest2/kubetest2-tester-ginkgo@latest

echo "Downloading/installing ginkgo binary"
go install github.com/onsi/ginkgo/v2/ginkgo@latest
cp "${GOBIN}/ginkgo" "${RUN_DIR}/ginkgo"

echo "Downloading installing kubectl (${VERSION})"
curl -LO -s "https://dl.k8s.io/release/${VERSION}/bin/linux/amd64/kubectl"
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

# Install bazel
#
BAZEL_VERSION="5.4.0"
if [[ -f "${REPO_ROOT}/.bazelversion" ]]; then
    BAZEL_VERSION=$(cat "${REPO_ROOT}/.bazelversion")
fi
echo "BAZEL_VERSION set to ${BAZEL_VERSION}"
INSTALLER="bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh"
if [[ "${BAZEL_VERSION}" =~ ([0-9\.]+)(rc[0-9]+) ]]; then
    DOWNLOAD_URL="https://storage.googleapis.com/bazel/${BASH_REMATCH[1]}/${BASH_REMATCH[2]}/${INSTALLER}"
else
    DOWNLOAD_URL="https://github.com/bazelbuild/bazel/releases/download/${BAZEL_VERSION}/${INSTALLER}"
fi
echo "$DOWNLOAD_URL"
# get the installer
wget -q "${DOWNLOAD_URL}" && chmod +x "${INSTALLER}"
# install to /usr/local or user dir
"./${INSTALLER}" ${bazel_user}
# remove the installer, we no longer need it
rm "${INSTALLER}"

# Run the e2e test using the "gce" provider and the "ginkgo" tester.
echo "Running e2e test with k8s version: ${VERSION}"
kubetest2 gce \
	  -v ${VERBOSITY} \
	  --kubernetes-version ${VERSION} \
	  --repo-root ${REPO_ROOT} \
	  --run-id ${RUN_ID} ${gcp_project_args} \
	  --build \
	  --up \
	  --down \
	  --test ginkgo \
	  --master-size ${MASTER_SIZE} \
	  --node-size ${NODE_SIZE} \
	  --overwrite-logs-dir \
	  -- \
	  --use-built-binaries true \
	  --parallel 30 \
	  --test-args="${test_args}" \
	  --focus-regex='\[cloud-provider-gcp-e2e\]'

echo
echo "FINISHED Running cloud-provider-gcp e2e tests"
