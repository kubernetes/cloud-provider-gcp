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

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)

function kops_setup_env() {
    # Determine workspace and bindir
    WORKSPACE=$(cd "${REPO_ROOT}/.." && pwd)
    BINDIR=${WORKSPACE}/bin
    export PATH=${BINDIR}:${PATH}
    mkdir -p "${BINDIR}"

    # Default cluster name if not set
    if [[ -z "${CLUSTER_NAME:-}" ]]; then
        local script_name=$(basename "${0}" .sh)
        CLUSTER_NAME="${script_name}.k8s.local"
    fi
    echo "CLUSTER_NAME=${CLUSTER_NAME}"

    # Default workdir
    if [[ -z "${WORKDIR:-}" ]]; then
        WORKDIR="${WORKSPACE}/clusters/${CLUSTER_NAME}"
    fi
    mkdir -p "${WORKDIR}"
    export WORKSPACE BINDIR CLUSTER_NAME WORKDIR
    export KOPS_TEMPLATE_SRC="${REPO_ROOT}/test/kops-cluster.yaml.template"
    export CONTROL_PLANE_MACHINE_TYPE="${CONTROL_PLANE_MACHINE_TYPE:-e2-standard-2}"
    export NODE_MACHINE_TYPE="${NODE_MACHINE_TYPE:-e2-standard-2}"
    export NODE_COUNT="${NODE_COUNT:-1}"

    # Ensure we have a project; get one from boskos if one not provided in GCP_PROJECT
    source "${REPO_ROOT}/test/boskos.sh"
    if [[ -z "${GCP_PROJECT:-}" ]]; then
        echo "GCP_PROJECT not set, acquiring project from boskos"
        acquire_project
        GCP_PROJECT="${PROJECT}"
        CLEANUP_BOSKOS="true"
    fi
    echo "GCP_PROJECT=${GCP_PROJECT}"
    export GCP_PROJECT CLEANUP_BOSKOS

    # Ensure we have an SSH key; needed for node access and artifact collection
    if [[ -z "${SSH_PRIVATE_KEY:-}" ]]; then
        echo "SSH_PRIVATE_KEY not set, checking for default in WORKDIR"
        SSH_PRIVATE_KEY="${WORKDIR}/google_compute_engine"
        if [[ ! -f "${SSH_PRIVATE_KEY}" ]]; then
            echo "Creating new SSH keypair at ${SSH_PRIVATE_KEY}"
            gcloud compute --project="${GCP_PROJECT}" config-ssh --ssh-key-file="${SSH_PRIVATE_KEY}"
        fi
        export KUBE_SSH_USER="${USER}"
    fi
    echo "SSH_PRIVATE_KEY=${SSH_PRIVATE_KEY}"
    export SSH_PRIVATE_KEY

    echo "Installing kubetest2-kops..."
    pushd "${WORKSPACE}/kops" >/dev/null
    GOBIN=${BINDIR} make test-e2e-install
    popd >/dev/null

    if [[ -z "${K8S_VERSION:-}" ]]; then
        K8S_VERSION="$(curl -sL https://dl.k8s.io/release/stable.txt)"
    fi
    export K8S_VERSION

    # Download latest prebuilt kOps binary
    if [[ -z "${KOPS_BASE_URL:-}" ]]; then
        KOPS_BRANCH="master"
        KOPS_BASE_URL="$(curl -s https://storage.googleapis.com/k8s-staging-kops/kops/releases/markers/${KOPS_BRANCH}/latest-ci-updown-green.txt)"
    fi
    export KOPS_BASE_URL

    KOPS_BIN=${BINDIR}/kops
    if [[ ! -f "${KOPS_BIN}" ]]; then
        echo "Downloading kOps binary from ${KOPS_BASE_URL}"
        wget -qO "${KOPS_BIN}" "$KOPS_BASE_URL/$(go env GOOS)/$(go env GOARCH)/kops"
        chmod +x "${KOPS_BIN}"
    fi
    export KOPS_BIN

    # Set cloud provider to gce
    export CLOUD_PROVIDER="gce"

    # KOPS_STATE_STORE holds metadata about the clusters we create
    if [[ -z "${KOPS_STATE_STORE:-}" ]]; then
        KOPS_STATE_STORE="gs://kops-state-${GCP_PROJECT}"
        # Ensure the bucket exists
        gsutil ls -p "${GCP_PROJECT}" "${KOPS_STATE_STORE}" >/dev/null 2>&1 || \
            gsutil mb -p "${GCP_PROJECT}" -l "${GCP_LOCATION:-us-central1}" "${KOPS_STATE_STORE}"

        # Disable uniform bucket-level access so kOps can manage ACLs
        gsutil ubla set off "${KOPS_STATE_STORE}"

        # Grant storage.admin on the bucket to the current ServiceAccount
        SA=$(gcloud config list --format 'value(core.account)')
        gsutil iam ch serviceAccount:${SA}:admin "${KOPS_STATE_STORE}"
    fi
    echo "KOPS_STATE_STORE=${KOPS_STATE_STORE}"
    export KOPS_STATE_STORE


    export GCP_LOCATION="${GCP_LOCATION:-us-central1}"
    export GCP_ZONES="${GCP_ZONES:-${GCP_LOCATION}-b}"

    # Hydrate the template with environment variables
    # This creates a final Go-template that kOps can then render with its own values (like .zones)
    export KOPS_TEMPLATE="${WORKDIR}/kops-cluster.yaml"
    envsubst '$GCP_PROJECT $GCP_LOCATION $GCP_ZONES $NEW_CCM_SPEC $CONTROL_PLANE_MACHINE_TYPE $NODE_MACHINE_TYPE $NODE_COUNT' < "${KOPS_TEMPLATE_SRC}" > "${KOPS_TEMPLATE}"

    echo "--- Hydrated kOps Template (${KOPS_TEMPLATE}) ---"
    cat "${KOPS_TEMPLATE}"
    echo "--- End of Template ---"
}

# kops_build_and_push_images builds the Cloud Controller Manager image and pushes it to GCR.
function kops_build_and_push_images() {
    if [[ -z "${IMAGE_REPO:-}" ]]; then
        IMAGE_REPO="gcr.io/${GCP_PROJECT}"
    fi
    echo "IMAGE_REPO=${IMAGE_REPO}"
    export IMAGE_REPO

    if [[ -z "${IMAGE_TAG:-}" ]]; then
        IMAGE_TAG=$(git rev-parse --short HEAD)-$(date +%Y%m%dT%H%M%S)
    fi
    echo "IMAGE_TAG=${IMAGE_TAG}"
    export IMAGE_TAG

    pushd "${REPO_ROOT}" >/dev/null
    export KUBE_ROOT=${REPO_ROOT}
    source "${REPO_ROOT}/tools/version.sh"
    get_version_vars
    unset KUBE_ROOT

    echo "Configuring docker auth..."
    gcloud auth configure-docker --quiet

    echo "Building and pushing CCM image..."
    IMAGE_REPO=${IMAGE_REPO} IMAGE_TAG=${IMAGE_TAG} tools/push-images
    popd >/dev/null
}

# kops_create_cluster launches the kOps cluster using kubetest2.
function kops_create_cluster() {
    if [[ -z "${ADMIN_ACCESS:-}" ]]; then
        ADMIN_ACCESS="0.0.0.0/0"
    fi
    echo "ADMIN_ACCESS=${ADMIN_ACCESS}"

    KUBETEST2_ARGS=""
    KUBETEST2_ARGS="${KUBETEST2_ARGS} -v=2 --cloud-provider=${CLOUD_PROVIDER}"
    KUBETEST2_ARGS="${KUBETEST2_ARGS} --cluster-name=${CLUSTER_NAME}"
    KUBETEST2_ARGS="${KUBETEST2_ARGS} --kops-binary-path=${KOPS_BIN}"
    KUBETEST2_ARGS="${KUBETEST2_ARGS} --admin-access=${ADMIN_ACCESS}"
    KUBETEST2_ARGS="${KUBETEST2_ARGS} --env=KOPS_FEATURE_FLAGS=${KOPS_FEATURE_FLAGS:-}"

    if [[ -n "${GCP_PROJECT:-}" ]]; then
        KUBETEST2_ARGS="${KUBETEST2_ARGS} --gcp-project=${GCP_PROJECT}"
    fi

    if [[ -n "${SSH_PRIVATE_KEY:-}" ]]; then
        KUBETEST2_ARGS="${KUBETEST2_ARGS} --ssh-private-key=${SSH_PRIVATE_KEY}"
        KUBETEST2_ARGS="${KUBETEST2_ARGS} --ssh-public-key=${SSH_PRIVATE_KEY}.pub"
    fi

    if [[ -n "${GOOGLE_APPLICATION_CREDENTIALS:-}" ]]; then
        KUBETEST2_ARGS="${KUBETEST2_ARGS} --env=GOOGLE_APPLICATION_CREDENTIALS=${GOOGLE_APPLICATION_CREDENTIALS}"
    fi
    export KUBETEST2_ARGS

    if [[ "${CREATE_CLUSTER:-}" == "false" ]]; then
        echo "CREATE_CLUSTER is false, reusing existing cluster."
        return
    fi
    
    echo "Creating kOps cluster using template..."
    kubetest2 kops ${KUBETEST2_ARGS} \
        --up \
        --kubernetes-version="${K8S_VERSION}" \
        --template-path="${KOPS_TEMPLATE}"
}

# kops_delete_cluster tears down the kOps cluster.
function kops_delete_cluster() {
    echo "Deleting kOps cluster..."
    kubetest2 kops ${KUBETEST2_ARGS} --down || echo "Warning: kubetest2 kops --down failed"
}

# kops_cleanup is the standard cleanup handler for E2E tests.
function kops_cleanup {
    local exit_status=$?
    echo "Cleaning up E2E environment (Exit Status: ${exit_status})..."
    
    if [[ "${CLEANUP_BOSKOS:-}" == "true" ]]; then
        cleanup_boskos
    fi
    if [[ "${DELETE_CLUSTER:-}" == "true" ]]; then
        kops_delete_cluster
    fi
}
