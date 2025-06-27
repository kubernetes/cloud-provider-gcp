#!/usr/bin/env bash

# Copyright 2022 The Kubernetes Authors.
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

set -e
set -x

# This script brings up a Kubernetes cluster using kops and a local CCM image.
# It is based on the e2e test script in e2e/scenarios/kops-simple.

REPO_ROOT=$(git rev-parse --show-toplevel)
cd ${REPO_ROOT}
cd ..
WORKSPACE=$(pwd)

# Create bindir
BINDIR=${WORKSPACE}/bin
export PATH=${BINDIR}:${PATH}
mkdir -p "${BINDIR}"

# Setup our cleanup function; as we allocate resources we set a variable to indicate they should be cleaned up
function cleanup {
  if [[ "${DELETE_CLUSTER:-}" == "true" ]]; then
      kops delete cluster --name "${CLUSTER_NAME}" --state "${KOPS_STATE_STORE}" --yes || echo "kops delete cluster failed"
  fi
}
trap cleanup EXIT

# Default cluster name
SCRIPT_NAME=$(basename $0 .sh)
if [[ -z "${CLUSTER_NAME:-}" ]]; then
  CLUSTER_NAME="${SCRIPT_NAME}.k8s.local"
fi
echo "CLUSTER_NAME=${CLUSTER_NAME}"

# Default workdir
if [[ -z "${WORKDIR:-}" ]]; then
  WORKDIR="${WORKSPACE}/clusters/${CLUSTER_NAME}"
fi
mkdir -p "${WORKDIR}"

if [[ -z "${GCP_PROJECT:-}" ]]; then
  echo "GCP_PROJECT must be set"
  exit 1
fi
echo "GCP_PROJECT=${GCP_PROJECT}"

# Ensure we have an SSH key; needed to dump the node information to artifacts/
if [[ -z "${SSH_PRIVATE_KEY:-}" ]]; then
  echo "SSH_PRIVATE_KEY not set, creating one"

  SSH_PRIVATE_KEY="${WORKDIR}/google_compute_engine"
  # This will create a new key if one doesn't exist, and add it to the project metadata.
  gcloud compute config-ssh --project="${GCP_PROJECT}" --ssh-key-file="${SSH_PRIVATE_KEY}" --quiet
  export KUBE_SSH_USER="${USER}"
fi
echo "SSH_PRIVATE_KEY=${SSH_PRIVATE_KEY}"
export KUBE_SSH_PUBLIC_KEY_PATH="${SSH_PRIVATE_KEY}.pub"


if [[ -z "${K8S_VERSION:-}" ]]; then
  K8S_VERSION="$(curl -sL https://dl.k8s.io/release/stable.txt)"
fi

# Set cloud provider to gce
CLOUD_PROVIDER="gce"
echo "CLOUD_PROVIDER=${CLOUD_PROVIDER}"

#Set cloud provider location
GCP_LOCATION="${GCP_LOCATION:-us-central1}"
ZONES="${ZONES:-us-central1-a}"

# KOPS_STATE_STORE holds metadata about the clusters we create
if [[ -z "${KOPS_STATE_STORE:-}" ]]; then
  KOPS_STATE_STORE="gs://kops-state-${GCP_PROJECT}"
fi

# Ensure the bucket exists
if ! gsutil ls -p "${GCP_PROJECT}" "${KOPS_STATE_STORE}" >/dev/null 2>&1; then
  gsutil mb -p "${GCP_PROJECT}" -l "${GCP_LOCATION}" "${KOPS_STATE_STORE}"
  # Setting ubla off so that kOps can automatically set ACLs for the default serviceACcount
  gsutil ubla set off "${KOPS_STATE_STORE}"
fi

echo "KOPS_STATE_STORE=${KOPS_STATE_STORE}"
export KOPS_STATE_STORE

# IMAGE_REPO is used to upload images
if [[ -z "${IMAGE_REPO:-}" ]]; then
  IMAGE_REPO="gcr.io/${GCP_PROJECT}"
fi
echo "IMAGE_REPO=${IMAGE_REPO}"

cd ${REPO_ROOT}
if [[ -z "${IMAGE_TAG:-}" ]]; then
  IMAGE_TAG=$(git rev-parse --short HEAD)-$(date +%Y%m%dT%H%M%S)
fi
echo "IMAGE_TAG=${IMAGE_TAG}"

# Build and push cloud-controller-manager
cd ${REPO_ROOT}

export KUBE_ROOT=${REPO_ROOT}
source "${REPO_ROOT}/tools/version.sh"
get_version_vars
unset KUBE_ROOT

echo "git status:"
git status

echo "Configuring docker auth with gcloud"
gcloud auth configure-docker

echo "Building and pushing images"
IMAGE_REPO=${IMAGE_REPO} IMAGE_TAG=${IMAGE_TAG} tools/push-images

if [[ -z "${ADMIN_ACCESS:-}" ]]; then
  ADMIN_ACCESS="0.0.0.0/0" # Or use your IPv4 with /32
fi
echo "ADMIN_ACCESS=${ADMIN_ACCESS}"

# Add our manifest
cp "${REPO_ROOT}/deploy/packages/default/manifest.yaml" "${WORKDIR}/cloud-provider-gcp.yaml"
sed -i -e "s@k8scloudprovidergcp/cloud-controller-manager:latest@${IMAGE_REPO}/cloud-controller-manager:${IMAGE_TAG}@g" "${WORKDIR}/cloud-provider-gcp.yaml"

# Enable cluster addons, this enables us to replace the built-in manifest
export KOPS_FEATURE_FLAGS="ClusterAddons,${KOPS_FEATURE_FLAGS:-}"
echo "KOPS_FEATURE_FLAGS=${KOPS_FEATURE_FLAGS}"

# The caller can set DELETE_CLUSTER=false to stop us deleting the cluster
if [[ -z "${DELETE_CLUSTER:-}" ]]; then
  DELETE_CLUSTER="true"
fi

kops create cluster \
  --name "${CLUSTER_NAME}" \
  --state "${KOPS_STATE_STORE}" \
  --zones "${ZONES}" \
  --project "${GCP_PROJECT}" \
  --kubernetes-version="${K8S_VERSION}" \
  --node-count "${NODE_COUNT:-2}" \
  --node-size "${NODE_SIZE:-e2-medium}" \
  --master-size "${MASTER_SIZE:-e2-medium}" \
  --cloud-labels "Owner=${USER},ManagedBy=kops" \
  --networking "gce" \
  --gce-service-account="default" \
  --ssh-public-key="${KUBE_SSH_PUBLIC_KEY_PATH}" \
  --admin-access="${ADMIN_ACCESS}" \
  --add="${WORKDIR}/cloud-provider-gcp.yaml"

kops update cluster "${CLUSTER_NAME}" --state "${KOPS_STATE_STORE}" --yes

echo "Cluster is being created. It may take a few minutes."
echo "You can check the status with: kops validate cluster --name ${CLUSTER_NAME} --state ${KOPS_STATE_STORE}"

if [[ "${DELETE_CLUSTER:-}" == "true" ]]; then
  # Don't delete again in trap
  DELETE_CLUSTER=false
fi
