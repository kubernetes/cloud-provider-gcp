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
echo "SCRIPT_ROOT ${SCRIPT_ROOT}"
cd ${SCRIPT_ROOT}

readonly GO111MODULE="on"
readonly GOFLAGS="-mod=mod"
readonly GOPATH="$(mktemp -d)"

export GO111MODULE GOFLAGS GOPATH

# Even when modules are enabled, the code-generator tools always write to
# a traditional GOPATH directory, so fake on up to point to the current
# workspace.
mkdir -p "$GOPATH/src/k8s.io/cloud-provider-gcp"
ln -s "${SCRIPT_ROOT}" "$GOPATH/src/k8s.io/cloud-provider-gcp/crd"

# mkdir -p "$GOPATH/src/k8s.io"
# ln -s "${SCRIPT_ROOT}" "$GOPATH/src/k8s.io/cloud-provider-gcp"
echo " GOPATH/src/k8s.io/cloud-provider-gcp/crd ${GOPATH}/src/k8s.io/cloud-provider-gcp/crd"
cd ${SCRIPT_ROOT}

readonly REPO_BASE=k8s.io/cloud-provider-gcp/crd
readonly OUTPUT_BASE_PKG=${REPO_BASE}/client
readonly APIS_BASE_PKG=${REPO_BASE}/apis
readonly CLIENTSET_NAME=versioned
readonly CLIENTSET_PKG_NAME=clientset

if [[ "${VERIFY_CODEGEN:-}" == "true" ]]; then
  echo "Running in verification mode"
  readonly VERIFY_FLAG="--verify-only"
fi

readonly COMMON_FLAGS="${VERIFY_FLAG:-} --go-header-file ${SCRIPT_ROOT}/hack/boilerplate.go.txt"

generate_config() {
  local crd_name version crd_version apis_pkg
  crd_name="$1"
  version="$2"
  crd_version="$3"
  apis_pkg="${APIS_BASE_PKG}/${1}/${2}/..."

  if [[ $# -eq 4 ]]; then
    apis_pkg="{${apis_pkg},${APIS_BASE_PKG}/${1}/${4}/...}"
  fi

  echo "Performing code generation for ${crd_name} CRD"
  echo "Generating deepcopy functions and CRD artifacts"
  go run sigs.k8s.io/controller-tools/cmd/controller-gen \
          object:headerFile="${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
          crd:crdVersions="${crd_version}" \
          paths="${apis_pkg}" \
          output:crd:artifacts:config="${SCRIPT_ROOT}/config/crds"
}

codegen_for () {
  local crd_name version crd_version extra_version apis_pkg output_pkg

  if [[ $# -ne 3 ]] && [[ $# -ne 4 ]]; then
    echo "Usage: codegen_for CRD-NAME VERSION CRDSPEC-VERSION optional:EXTRA-VERSION" >&2
    echo "" >&2
    echo "This writes auto generated client methods for CRD-NAME/VERSIONs" >&2
    return 1
  fi

  crd_name="$1"
  version="$2"
  crd_version="$3"
  output_pkg="${OUTPUT_BASE_PKG}/${1}"
  apis_pkg="${APIS_BASE_PKG}/${1}/${2}"

  if [[ $# -eq 4 ]]; then
    extra_version="$4"
    apis_pkg="${apis_pkg},${APIS_BASE_PKG}/${1}/${extra_version}"
    generate_config "$crd_name" "$version" "$crd_version" "$extra_version"
  else
    generate_config "$crd_name" "$version" "$crd_version"
  fi


  echo "Generating clientset at ${output_pkg}/${CLIENTSET_PKG_NAME}"
  echo "apis_pkg ${apis_pkg}"
  echo "output_pkg/CLIENTSET_PKG_NAME ${output_pkg}/${CLIENTSET_PKG_NAME}"
  go run k8s.io/code-generator/cmd/client-gen \
          --input-base "" \
          --input "${apis_pkg}" \
          --clientset-name "${CLIENTSET_NAME}" \
          --output-package "${output_pkg}/${CLIENTSET_PKG_NAME}" \
          ${COMMON_FLAGS}

  echo "Generating listers at ${output_pkg}/listers"
  go run k8s.io/code-generator/cmd/lister-gen \
          --input-dirs "${apis_pkg}" \
          --output-package "${output_pkg}/listers" \
          ${COMMON_FLAGS}

  echo "Generating informers at ${output_pkg}/informers"
  go run k8s.io/code-generator/cmd/informer-gen \
           --input-dirs "${apis_pkg}" \
           --versioned-clientset-package "${output_pkg}/${CLIENTSET_PKG_NAME}/${CLIENTSET_NAME}" \
           --listers-package "${output_pkg}/listers" \
           --output-package "${output_pkg}/informers" \
           ${COMMON_FLAGS}

  echo "Generating register at ${apis_pkg}"
  go run k8s.io/code-generator/cmd/register-gen \
          --input-dirs "${apis_pkg}" \
          --output-package "${apis_pkg}" \
          ${COMMON_FLAGS}
}

codegen_for network v1alpha1 v1 v1
codegen_for gcpfirewall v1beta1 v1
