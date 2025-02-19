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

set -o xtrace
set -o errexit
set -o nounset
set -o pipefail

cd $(mktemp -d)
KUBE_VERSION="${KUBE_VERSION:-v1.30.1}"

WORKSPACE_TARGETS=(
  "kubernetes-server-linux-amd64.tar.gz"
  "kubernetes-manifests.tar.gz"
  "kubernetes-node-linux-amd64.tar.gz"
  "kubernetes-node-windows-amd64.tar.gz"
)

for t in "${WORKSPACE_TARGETS[@]}"; do
    wget "https://dl.k8s.io/${KUBE_VERSION}/$t"
done

sha256sum "${WORKSPACE_TARGETS[@]}"
