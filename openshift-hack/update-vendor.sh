#!/usr/bin/env bash

# Copyright 2018 The Kubernetes Authors.
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

cd "$(pwd -P)"

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# rebuild go.work
cat go.mod | grep '^go' > go.work
go work use .
go work use providers
# Note: we can't add test/e2e to go.work because of getting dependency issues otherwise.
# go work use test/e2e
echo -e "\nreplace (" >> go.work
cat go.mod | grep '=>' | sort | uniq | \
    grep -v "k8s.io/cloud-provider-gcp/providers" | \
    sed 's/replace /\t/' \
    >> go.work
echo -e ")" >> go.work

# sync go.md of providers
go work sync

# update vendor/
go work vendor

# clean up unused dependencies
cd providers
go mod tidy
cd ..

go mod tidy
