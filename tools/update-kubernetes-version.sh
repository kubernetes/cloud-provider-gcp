#!/usr/bin/env bash

# Copyright 2023 The Kubernetes Authors.
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

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KUBE_VERSION=${KUBE_VERSION:-master}
KUBE_SOURCE=_src/k8s.io/kubernetes

# The scripts uses go workspaces to use the local modules instead of the referenced by go.mod
# It clones the kubernetes version specified to use it instead of the one in the vendor repository

cd $KUBE_ROOT
# remove any kubernetes reference from go.mod
sed -i 's/^\s*k8s.io.*//g' go.mod
# clone kubernetes version
rm -rf ${KUBE_SOURCE}
git clone --depth 1 --branch ${KUBE_VERSION} https://github.com/kubernetes/kubernetes ${KUBE_SOURCE}
# Create the workspaces for each module 
go work init .
go work use crd
go work use providers
go work use ${KUBE_SOURCE}
for i in ${KUBE_SOURCE}/staging/src/k8s.io/*; do 
    go work use ./$i
done
go work sync

go list all