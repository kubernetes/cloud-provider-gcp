#!/bin/bash

# Copyright 2021 The Kubernetes Authors.
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

readonly SCRIPT_ROOT=$(cd $(dirname ${BASH_SOURCE})/.. && pwd)
readonly CODEGEN_PKG=${SCRIPT_ROOT}/vendor/k8s.io/code-generator
readonly CONTROLLER_GEN=${SCRIPT_ROOT}/vendor/sigs.k8s.io/controller-tools

readonly GO111MODULE="on"
readonly GOFLAGS="-mod=vendor"
readonly GOPATH="$(mktemp -d)"

export GO111MODULE GOFLAGS GOPATH

# Even when modules are enabled, the code-generator tools always write to
# a traditional GOPATH directory, so fake on up to point to the current
# workspace.
mkdir -p "$GOPATH/src/k8s.io"
ln -s "${SCRIPT_ROOT}" "$GOPATH/src/k8s.io/cloud-provider-gcp"

readonly REPO_BASE=k8s.io/cloud-provider-gcp
readonly OUTPUT_BASE_PKG=${REPO_BASE}/pkg/client
readonly APIS_BASE_PKG=${REPO_BASE}/pkg/apis
readonly CLIENTSET_NAME=versioned
readonly CLIENTSET_PKG_NAME=clientset

if [[ "${VERIFY_CODEGEN:-}" == "true" ]]; then
  echo "Running in verification mode"
  readonly VERIFY_FLAG="--verify-only"
fi

readonly COMMON_FLAGS="${VERIFY_FLAG:-} --go-header-file ${SCRIPT_ROOT}/hack/boilerplate.go.txt"

codegen_for () {
  local crd_name version apis_pkg output_pkg

  if [[ $# != 2 ]]; then
    echo "Usage: codegen_for CRD-NAME VERSION" >&2
    echo "" >&2
    echo "This writes auto generated client methods for CRD-NAME/VERSION" >&2
    return 1
  fi

  crd_name=${1}
  version=${2}
  output_pkg=${OUTPUT_BASE_PKG}/${1}
  apis_pkg=${APIS_BASE_PKG}/${1}/${2}

  echo "Performing code generation for ${crd_name} CRD"
  echo "Generating deepcopy functions and CRD artifacts"
  go run ${CONTROLLER_GEN}/cmd/controller-gen \
          object:headerFile=${SCRIPT_ROOT}/hack/boilerplate.go.txt \
          crd:crdVersions=v1 \
          paths=${apis_pkg}/... \
          output:crd:artifacts:config=${SCRIPT_ROOT}/config/crds

  echo "Generating clientset at ${output_pkg}/${CLIENTSET_PKG_NAME}"
  go run ${CODEGEN_PKG}/cmd/client-gen \
          --clientset-name "${CLIENTSET_NAME}" \
          --input-base "" \
          --input "${apis_pkg}" \
          --output-package "${output_pkg}/${CLIENTSET_PKG_NAME}" \
          ${COMMON_FLAGS}

  echo "Generating listers at ${output_pkg}/listers"
  go run ${CODEGEN_PKG}/cmd/lister-gen \
          --input-dirs "${apis_pkg}" \
          --output-package "${output_pkg}/listers" \
          ${COMMON_FLAGS}

  echo "Generating informers at ${output_pkg}/informers"
  go run ${CODEGEN_PKG}/cmd/informer-gen \
           --input-dirs "${apis_pkg}" \
           --versioned-clientset-package "${output_pkg}/${CLIENTSET_PKG_NAME}/${CLIENTSET_NAME}" \
           --listers-package "${output_pkg}/listers" \
           --output-package "${output_pkg}/informers" \
           ${COMMON_FLAGS}

  echo "Generating register at ${apis_pkg}"
  go run ${CODEGEN_PKG}/cmd/register-gen \
          --input-dirs "${apis_pkg}" \
          --output-package "${apis_pkg}" \
          ${COMMON_FLAGS}
}

codegen_for gcpfirewall v1alpha1