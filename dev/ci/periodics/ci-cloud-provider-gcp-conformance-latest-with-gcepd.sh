#!/bin/bash

# Copyright 2026 The Kubernetes Authors.
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

# TODO: Use published release tars for cloud-provider-gcp if/once they exist
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

cd $GOPATH/src/k8s.io/cloud-provider-gcp
e2e/add-kubernetes-to-workspace.sh

export KOPS_FOCUS_REGEX='\[Conformance\]'
export KOPS_SKIP_REGEX='\[Serial\]'
export CLUSTER_NAME="kops-conformance-gcepd.k8s.local"
export TEST_ARGS="--enabled-volume-drivers=gcepd"

e2e/scenarios/kops-simple
