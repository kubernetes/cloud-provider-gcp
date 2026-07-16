#!/usr/bin/env bash

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

# This script checks coding style for go language files using golangci-lint.
# Usage: `tools/verify-golangci-lint.sh`.

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${KUBE_ROOT}/tools/lib/util.sh"

KUBE_ROOT_ABSOLUTE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
export GOBIN="${KUBE_ROOT_ABSOLUTE}/bin"
export PATH="${GOBIN}:${PATH}"

if ! command -v golangci-lint &>/dev/null; then
  echo "golangci-lint not found. Installing via make..."
  make -C "${KUBE_ROOT}" golangci-lint
fi

cd "${KUBE_ROOT}"

failure_file="${KUBE_ROOT}/tools/.golangci-lint_failures"
kube::util::check-file-in-alphabetical-order "${failure_file}"

export IFS=$'\n'
# NOTE: when "go list -e ./..." is run within GOPATH, it turns the k8s.io/cloud-provider-gcp
# as the prefix, however if we run it outside it returns the full path of the file
# with a leading underscore. We'll need to support both scenarios for all_packages.
all_packages=()
while IFS='' read -r line; do all_packages+=("${line}"); done < <(go list -e ./... | grep -vE "/vendor" | sed -e 's|^k8s.io/cloud-provider-gcp/||' -e "s|^_\(${KUBE_ROOT}/\)\{0,1\}||")

# Read failing packages
failing_packages=()
while IFS='' read -r line; do
  # skip comments and empty lines
  if [[ -n "${line}" ]] && [[ ! "${line}" =~ ^# ]]; then
    failing_packages+=("${line}")
  fi
done < "${failure_file}"

errors=()
not_failing=()

for p in "${all_packages[@]}"; do
  kube::util::array_contains "${p}" "${failing_packages[@]}" && in_failing=$? || in_failing=$?

  # Run golangci-lint on the package directory
  # To make it package-focused, we pass the package directory to golangci-lint
  if failedLint=$(golangci-lint run --config="${KUBE_ROOT_ABSOLUTE}/.golangci.yml" "${p}" 2>/dev/null); then
    failedLint=""
  fi

  if [[ -n "${failedLint}" ]] && [[ "${in_failing}" -ne "0" ]]; then
    errors+=("${failedLint}")
  fi
  if [[ -z "${failedLint}" ]] && [[ "${in_failing}" -eq "0" ]]; then
    not_failing+=("${p}")
  fi
done

# Check that all failing_packages actually still exist
gone=()
for p in "${failing_packages[@]}"; do
  kube::util::array_contains "${p}" "${all_packages[@]}" || gone+=("${p}")
done

# Check to be sure all the packages that should pass lint are.
if [ ${#errors[@]} -eq 0 ]; then
  echo 'Congratulations! All Go source files have been linted.'
else
  {
    echo "Errors from golangci-lint:"
    for err in "${errors[@]}"; do
      echo "${err}"
    done
    echo
    echo 'Please review the above warnings. You can test via "golangci-lint run" and commit the result.'
    echo 'If the above warnings do not make sense, you can exempt this package from linting'
    echo 'checking by adding it to tools/.golangci-lint_failures.'
    echo
  } >&2
  exit 1
fi

if [[ ${#not_failing[@]} -gt 0 ]]; then
  {
    echo "Some packages in tools/.golangci-lint_failures are passing golangci-lint. Please remove them:"
    echo
    for p in "${not_failing[@]}"; do
      echo "  ${p}"
    done
    echo
  } >&2
  exit 1
fi

if [[ ${#gone[@]} -gt 0 ]]; then
  {
    echo "Some packages in tools/.golangci-lint_failures do not exist anymore. Please remove them:"
    echo
    for p in "${gone[@]}"; do
      echo "  ${p}"
    done
    echo
  } >&2
  exit 1
fi
