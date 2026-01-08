#!/usr/bin/env bash

# Copyright 2025 The Kubernetes Authors.
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

# This script deploys a Kubernetes cluster using kops with configurable version targets.
# Modes:
#   lkg-k8s-local-gcp: LKG K8s version + Local CCM build
#   latest-k8s-lkg-gcp: Latest Stable K8s + Stock/LKG CCM (Kops default) (Note: This requires kops to support the latest K8s version; kops releases may lag behind upstream)
#   stock: Stock Kops behavior (Kops default K8s + Kops default CCM)
#
# Cluster Lifecycle:
#   By default (DELETE_CLUSTER=true), the cluster is deleted if this script fails or is interrupted.
#   On success, the cluster is PRESERVED (not deleted) to allow for testing/debugging.
#   Set DELETE_CLUSTER=false to preserve the cluster even on failure.

set -o errexit
set -o nounset
set -o pipefail
set -x

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

usage() {
  echo "Usage: $0 --mode <lkg-k8s-local-gcp|latest-k8s-lkg-gcp|stock>"
  echo ""
  echo "Environment variables:"
  echo "  GCP_PROJECT       (Required) GCP Project ID"
  echo "  CLUSTER_NAME      (Required) Cluster name (e.g. my-cluster.k8s.local)"
  echo "  DELETE_CLUSTER    (Optional) Default 'true': delete on failure, keep on success. Set 'false' to always keep."
  echo "  KOPS_STATE_STORE  (Optional) GCS bucket for kops state"
  echo "  GCP_LOCATION      (Optional) Region (default: us-central1)"
  echo "  ZONES             (Optional) Zones (default: us-central1-a)"
  exit 1
}

MODE=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --mode)
      MODE="$2"
      shift # past argument
      shift # past value
      ;;
    *)
      echo "Unknown option $1"
      usage
      ;;
  esac
done

if [[ -z "${MODE}" ]]; then
  usage
fi

if [[ -z "${GCP_PROJECT:-}" ]]; then
  echo "GCP_PROJECT must be set"
  exit 1
fi

if [[ -z "${CLUSTER_NAME:-}" ]]; then
  echo "CLUSTER_NAME must be set"
  exit 1
fi

# Default configuration
GCP_LOCATION="${GCP_LOCATION:-us-central1}"
ZONES="${ZONES:-us-central1-a}"
NODE_COUNT="${NODE_COUNT:-2}"
NODE_SIZE="${NODE_SIZE:-e2-medium}"
MASTER_SIZE="${MASTER_SIZE:-e2-medium}"
DELETE_CLUSTER="${DELETE_CLUSTER:-true}"

# Setup KOPS_STATE_STORE
if [[ -z "${KOPS_STATE_STORE:-}" ]]; then
  KOPS_STATE_STORE="gs://kops-state-${GCP_PROJECT}"
fi
export KOPS_STATE_STORE

# Ensure bucket exists
if ! gsutil ls -p "${GCP_PROJECT}" "${KOPS_STATE_STORE}" >/dev/null 2>&1; then
  gsutil mb -p "${GCP_PROJECT}" -l "${GCP_LOCATION}" "${KOPS_STATE_STORE}"
  gsutil ubla set off "${KOPS_STATE_STORE}"
fi

# SSH Key Setup
if [[ -z "${SSH_PRIVATE_KEY_PATH:-}" ]]; then
  SSH_PRIVATE_KEY_PATH="${REPO_ROOT}/google_compute_engine"
  if [[ ! -f "${SSH_PRIVATE_KEY_PATH}" ]]; then
      gcloud compute config-ssh --project="${GCP_PROJECT}" --ssh-key-file="${SSH_PRIVATE_KEY_PATH}" --quiet
  fi
  export KUBE_SSH_USER="${USER}"
fi
export KUBE_SSH_PUBLIC_KEY_PATH="${SSH_PRIVATE_KEY_PATH}.pub"

# Cleanup trap
function cleanup {
  if [[ "${DELETE_CLUSTER}" == "true" ]]; then
      echo "Deleting cluster..."
      kops delete cluster --name "${CLUSTER_NAME}" --yes || echo "kops delete cluster failed"
  fi
}
trap cleanup EXIT

# Mode-specific logic
BUILD_LOCAL_CCM=false
K8S_VERSION_ARG=""

case "${MODE}" in
  lkg-k8s-local-gcp)
    # Fetch LKG version from file
    LKG_FILE="${REPO_ROOT}/KUBERNETES_LKG"
    if [[ ! -f "${LKG_FILE}" ]]; then
        echo "ERROR: LKG file not found at ${LKG_FILE}"
        echo "Please run tools/update-lkg.sh to generate it."
        exit 1
    fi
    K8S_VERSION=$(cat "${LKG_FILE}")
    echo "Using LKG K8s Version: ${K8S_VERSION}"
    K8S_VERSION_ARG="--kubernetes-version=${K8S_VERSION}"
    BUILD_LOCAL_CCM=true
    ;;
  latest-k8s-lkg-gcp)
    echo "Fetching latest stable K8s version..."
    K8S_VERSION=$(curl -sL https://dl.k8s.io/release/stable.txt)
    echo "Using Latest Stable K8s Version: ${K8S_VERSION}"
    K8S_VERSION_ARG="--kubernetes-version=${K8S_VERSION}"
    BUILD_LOCAL_CCM=false
    ;;
  stock)
    echo "Using Stock Kops configuration..."
    K8S_VERSION_ARG="" # Let kops decide (or pass if user provided K8S_VERSION env var)
    if [[ -n "${K8S_VERSION:-}" ]]; then
        K8S_VERSION_ARG="--kubernetes-version=${K8S_VERSION}"
    fi
    BUILD_LOCAL_CCM=false
    ;;
  *)
    echo "Invalid mode: ${MODE}"
    usage
    ;;
esac

# Build Local CCM if needed
ADD_MANIFEST_ARG=""
if [[ "${BUILD_LOCAL_CCM}" == "true" ]]; then
    echo "Building Local CCM..."
    
    # Setup image tags
    if [[ -z "${IMAGE_REPO:-}" ]]; then
      IMAGE_REPO="gcr.io/${GCP_PROJECT}"
    fi
    if [[ -z "${IMAGE_TAG:-}" ]]; then
      IMAGE_TAG=$(git rev-parse --short HEAD)-$(date +%Y%m%dT%H%M%S)
    fi
    
    # Configure docker auth
    gcloud auth configure-docker --quiet
    
    # Build and Push
    IMAGE_REPO=${IMAGE_REPO} IMAGE_TAG=${IMAGE_TAG} "${REPO_ROOT}/tools/push-images"
    
    # Prepare Manifest
    WORKDIR="${REPO_ROOT}/_tmp/${CLUSTER_NAME}"
    mkdir -p "${WORKDIR}"
    cp "${REPO_ROOT}/deploy/packages/default/manifest.yaml" "${WORKDIR}/cloud-provider-gcp.yaml"
    sed -i -e "s@k8scloudprovidergcp/cloud-controller-manager:latest@${IMAGE_REPO}/cloud-controller-manager:${IMAGE_TAG}@g" "${WORKDIR}/cloud-provider-gcp.yaml"

    # Inject CCM args
    # We replace "args: [] ..." with the actual list of arguments required for CCM to run.
    sed -i -e "s|args: \[\] .*|args:\n          - --cloud-provider=gcp\n          - --leader-elect=true\n          - --use-service-account-credentials\n          - --allocate-node-cidrs=true\n          - --configure-cloud-routes=true\n          - --cluster-name=${CLUSTER_NAME}|" "${WORKDIR}/cloud-provider-gcp.yaml"
    
    ADD_MANIFEST_ARG="--add=${WORKDIR}/cloud-provider-gcp.yaml"
    
    # Enable addons
    export KOPS_FEATURE_FLAGS="ClusterAddons,${KOPS_FEATURE_FLAGS:-}"
fi

# Setup admin access
ADMIN_ACCESS="${ADMIN_ACCESS:-0.0.0.0/0}"

# Create Cluster
echo "Creating cluster with:"
echo "  Mode: ${MODE}"
echo "  Version Arg: ${K8S_VERSION_ARG}"
echo "  Manifest: ${ADD_MANIFEST_ARG}"

kops create cluster \
  --name "${CLUSTER_NAME}" \
  --zones "${ZONES}" \
  --project "${GCP_PROJECT}" \
  ${K8S_VERSION_ARG} \
  --node-count "${NODE_COUNT}" \
  --node-size "${NODE_SIZE}" \
  --master-size "${MASTER_SIZE}" \
  --cloud-labels "Owner=${USER},ManagedBy=kops,Mode=${MODE}" \
  --networking "gce" \
  --gce-service-account="default" \
  --ssh-public-key="${KUBE_SSH_PUBLIC_KEY_PATH}" \
  --admin-access="${ADMIN_ACCESS}" \
  ${ADD_MANIFEST_ARG}

kops update cluster "${CLUSTER_NAME}" --yes

echo "Cluster creation initiated. Waiting for readiness..."
# Validate the cluster and wait for up to 15 minutes for it to become ready
kops validate cluster --name "${CLUSTER_NAME}" --wait 15m

if [[ "${DELETE_CLUSTER}" == "true" ]]; then
    # Success! Disable the cleanup trap so the cluster persists.
    # The trap only deletes the cluster if DELETE_CLUSTER is still true (meaning script failed or was interrupted).
    DELETE_CLUSTER="false"
fi
