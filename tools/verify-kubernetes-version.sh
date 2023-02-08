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

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE}")/..
_tmp="$(mktemp -d -t "cloud-provider-gcp.XXXXXX")"

cleanup() {
 git worktree remove -f "${_tmp}"
}

trap "cleanup" EXIT SIGINT

git worktree add -f -q "${_tmp}" HEAD
cd "${_tmp}"

# Test if go modules work against latest kubernetes version

if ! tools/update-kubernetes-version.sh ; then
  echo "Latest version of Kubernetes has dependencies problems" >&2
  exit 1
fi

echo "Current Kubernetes version dependencies are correct"
echo