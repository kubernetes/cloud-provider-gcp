#!/usr/bin/env bash

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

KUBE_ROOT=$(dirname "${BASH_SOURCE}")/..

cd "${KUBE_ROOT}"

# Use eval to preserve embedded quoted strings.
eval "goflags=(${GOFLAGS:-})"

# Filter out arguments that start with "-" and move them to goflags.
targets=()
for arg; do
  if [[ "${arg}" == -* ]]; then
    goflags+=("${arg}")
  else
    targets+=("${arg}")
  fi
done

if [[ ${#targets[@]} -eq 0 ]]; then
  # Do not run on vendor directories.
  targets=$(go list ./... | grep -v "vendor")
fi

for i in $targets;do
  # Run go vet using goflags for each target specified.
  #
  # Remove any lines go vet or godep outputs with the exit status.
  # Remove any lines godep outputs about the vendor experiment.
  #
  # If go vet fails (produces output), grep will succeed, but if go vet
  # succeeds (produces no output) grep will fail. Then we just use
  # PIPESTATUS[0] which is go's exit code.
  #
  # The intended result is that each incantation of this line returns
  # either 0 (pass) or 1 (fail).
  echo "running $i"
  go vet "${goflags[@]:+${goflags[@]}}" "$i"
done


