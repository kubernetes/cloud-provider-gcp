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

# tools/kops-common.sh
# Shared functions for Kops deployment scripts

# Configures the KOPS_STATE_STORE bucket.
# Requires: GCP_PROJECT, GCP_LOCATION
# Sets/Exports: KOPS_STATE_STORE
function setup_kops_state_store() {
  if [[ -z "${KOPS_STATE_STORE:-}" ]]; then
    KOPS_STATE_STORE="gs://kops-state-${GCP_PROJECT}"
    echo "KOPS_STATE_STORE not set, using default: ${KOPS_STATE_STORE}"
  fi
  export KOPS_STATE_STORE

  # Ensure bucket exists
  if ! gsutil ls -p "${GCP_PROJECT}" "${KOPS_STATE_STORE}" >/dev/null 2>&1; then
    echo "Creating state store bucket: ${KOPS_STATE_STORE}"
    gsutil mb -p "${GCP_PROJECT}" -l "${GCP_LOCATION}" "${KOPS_STATE_STORE}"
    gsutil ubla set off "${KOPS_STATE_STORE}"
    
    # Grant storage.admin to the current service account (useful for CI/Boskos)
    local SA
    SA=$(gcloud config list --format 'value(core.account)')
    if [[ -n "${SA}" ]]; then
        echo "Granting admin access to ${SA}"
        gsutil iam ch serviceAccount:${SA}:admin "${KOPS_STATE_STORE}" || echo "Warning: Failed to grant IAM, possibly already owner or not a service account."
    fi
  fi
}

# Configures SSH keys for cluster access.
# Requires: GCP_PROJECT, WORKDIR/REPO_ROOT
# Sets/Exports: KUBE_SSH_USER, SSH_PRIVATE_KEY_PATH (or SSH_PRIVATE_KEY for kubetest2 compatibility)
function setup_ssh_key() {
  # Accept SSH_PRIVATE_KEY_PATH (dev script) or SSH_PRIVATE_KEY (CI script)
  local KEY_PATH="${SSH_PRIVATE_KEY_PATH:-${SSH_PRIVATE_KEY:-}}"
  
  if [[ -z "${KEY_PATH}" ]]; then
      # Default location
      KEY_PATH="${REPO_ROOT}/google_compute_engine"
      echo "SSH key path not set, using default: ${KEY_PATH}"
      
      if [[ ! -f "${KEY_PATH}" ]]; then
          echo "Generaing/Configuring SSH key..."
          gcloud compute config-ssh --project="${GCP_PROJECT}" --ssh-key-file="${KEY_PATH}" --quiet
      fi
      export KUBE_SSH_USER="${USER}"
  fi
  
  # Normalize variables for both scripts
  export SSH_PRIVATE_KEY="${KEY_PATH}"
  export SSH_PRIVATE_KEY_PATH="${KEY_PATH}"
  export KUBE_SSH_PUBLIC_KEY_PATH="${KEY_PATH}.pub"
  
  echo "SSH Key configured: ${KEY_PATH}"
}

# Builds CCM image and generates the manifest with arguments.
# Requires: REPO_ROOT, GCP_PROJECT, CLUSTER_NAME, WORKDIR
# Sets/Exports: ADD_MANIFEST_ARG, KOPS_FEATURE_FLAGS
function build_and_push_ccm() {
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
    echo "Pushing image to ${IMAGE_REPO}/cloud-controller-manager:${IMAGE_TAG}"
    IMAGE_REPO=${IMAGE_REPO} IMAGE_TAG=${IMAGE_TAG} "${REPO_ROOT}/tools/push-images"
    
    # Prepare Manifest
    local MANIFEST_DIR="${WORKDIR:-${REPO_ROOT}/_tmp/${CLUSTER_NAME}}"
    if [[ ! -d "${MANIFEST_DIR}" ]]; then
        mkdir -p "${MANIFEST_DIR}"
    fi
    
    echo "Generating manifest in ${MANIFEST_DIR}..."
    cp "${REPO_ROOT}/deploy/packages/default/manifest.yaml" "${MANIFEST_DIR}/cloud-provider-gcp.yaml"
    sed -i -e "s@k8scloudprovidergcp/cloud-controller-manager:latest@${IMAGE_REPO}/cloud-controller-manager:${IMAGE_TAG}@g" "${MANIFEST_DIR}/cloud-provider-gcp.yaml"

    # Inject CCM args
    # We replace "args: [] ..." with the actual list of arguments required for CCM to run.
    sed -i -e "s|args: \[\] .*|args:\n          - --cloud-provider=gcp\n          - --leader-elect=true\n          - --use-service-account-credentials\n          - --allocate-node-cidrs=true\n          - --configure-cloud-routes=true\n          - --cluster-name=${CLUSTER_NAME}|" "${MANIFEST_DIR}/cloud-provider-gcp.yaml"
    
    echo "Manifest generated at ${MANIFEST_DIR}/cloud-provider-gcp.yaml"
    
    # Export for use in calling script
    export ADD_MANIFEST_ARG="--add=${MANIFEST_DIR}/cloud-provider-gcp.yaml"
    export KOPS_FEATURE_FLAGS="ClusterAddons,${KOPS_FEATURE_FLAGS:-}"
}
