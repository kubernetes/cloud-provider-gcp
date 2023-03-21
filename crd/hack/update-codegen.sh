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

echo "GOPATH/src/k8s.io/cloud-provider-gcp/crd ${GOPATH}/src/k8s.io/cloud-provider-gcp/crd"


echo "Generating network CRD clientset"
"${SCRIPT_ROOT}/hack/generate-groups.sh" all \
  k8s.io/cloud-provider-gcp/crd/client/network \
  k8s.io/cloud-provider-gcp/crd/apis \
  "network:v1alpha1,v1" \
  --go-header-file "${SCRIPT_ROOT}/hack/boilerplate.go.txt"

echo "Generating firewall CRD clientset"
"${SCRIPT_ROOT}/hack/generate-groups.sh" all \
  k8s.io/cloud-provider-gcp/crd/client/gcpfirewall \
  k8s.io/cloud-provider-gcp/crd/apis \
  "gcpfirewall:v1beta1" \
  --go-header-file "${SCRIPT_ROOT}/hack/boilerplate.go.txt"


echo "Generating CRD artifacts"
go run sigs.k8s.io/controller-tools/cmd/controller-gen crd \
        object:headerFile="${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
        paths="${SCRIPT_ROOT}/apis/..." \
        output:crd:artifacts:config="${SCRIPT_ROOT}/config/crds"
