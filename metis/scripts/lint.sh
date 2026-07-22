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

set -o errexit
set -o nounset
set -o pipefail

METIS_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "${METIS_ROOT}"

# Ensure we use the correct Go version and binaries
export GOBIN="${METIS_ROOT}/bin"
export PATH="${GOBIN}:${PATH}"

# Install tools locally
if ! command -v staticcheck &> /dev/null || ! command -v revive &> /dev/null; then
  echo "Installing tools..."
  mkdir -p "${GOBIN}"
  pushd tools >/dev/null
    GOWORK=off go install github.com/mgechev/revive
    GOWORK=off go install honnef.co/go/tools/cmd/staticcheck
  popd >/dev/null
fi

res=0

echo "Running staticcheck..."
# Use if directly to capture output and handle exit code without triggering set -e
if SC_OUT=$(staticcheck ./... 2>&1); then
  echo "Passed."
else
  # Filter out the specific deprecation warning for NewSimpleClientset
  # TODO(https://github.com/kubernetes/cloud-provider-gcp/issues/1262): Remove this once upstream generates apply configs.
  FILTERED=$(echo "${SC_OUT}" | grep -v "NewSimpleClientset is deprecated" || true)

  if [[ -z "${FILTERED}" ]]; then
    echo "Passed (ignoring exempted warnings)."
  else
    echo "Failed. Output:"
    echo "${FILTERED}"
    res=1
  fi
fi



echo "Running revive..."
revive \
  -set_exit_status=1 \
  -exclude "**/*.pb.go" \
  -exclude "**/*_grpc.pb.go" \
  -formatter plain \
  -config tools/revive.toml \
  ./... || res=1

# Get list of Go files that are tracked and not generated
all_go_files=$(git ls-files | grep '\.go$' | grep -v '^_tmp/' | xargs grep -L "DO NOT EDIT" || true)

if [[ -n "${all_go_files}" ]]; then
  # 1. Copyright check (all files must have copyright)
  echo -n "Checking copyright headers... "
  missing_copyright=""
  for file in ${all_go_files}; do
    if ! grep -q "Copyright [0-9]\{4\} The Kubernetes Authors" "${file}"; then
      missing_copyright="${missing_copyright}${file}\n"
    fi
  done
  if [[ -n "${missing_copyright}" ]]; then
    echo "Failed. Files missing copyright header:"
    echo -e "${missing_copyright}"
    res=1
  else
    echo "Passed."
  fi

  # 2. Copyright year check (NEW files must have CURRENT year)
  echo -n "Checking copyright year for new files... "
  current_year=$(date +%Y)
  merge_base=$(git merge-base master HEAD 2>/dev/null || echo "master")
  # Find files added in this branch compared to master
  new_go_files=$(git diff --name-only --diff-filter=A ${merge_base} -- '*.go' | grep -v '^_tmp/' | xargs grep -L "DO NOT EDIT" 2>/dev/null || true)
  
  wrong_year_files=""
  for file in ${new_go_files}; do
    if [[ -f "${file}" ]]; then
      if ! grep -q "Copyright ${current_year} The Kubernetes Authors" "${file}"; then
        wrong_year_files="${wrong_year_files}${file} (expected year ${current_year})\n"
      fi
    fi
  done
  if [[ -n "${wrong_year_files}" ]]; then
    echo "Failed. New files must have current year copyright:"
    echo -e "${wrong_year_files}"
    res=1
  else
    echo "Passed."
  fi
fi

# 3. Trailing whitespace check
echo -n "Checking for trailing whitespace... "
# git grep returns non-zero if no matches, so we use || true
trailing_ws=$(git grep -n '[[:blank:]]$' -- '*.go' '*.md' | grep -v 'DO NOT EDIT' || true)
if [[ -n "${trailing_ws}" ]]; then
  echo "Failed. Trailing whitespace found:"
  echo "${trailing_ws}"
  res=1
else
  echo "Passed."
fi

# 4. Terminating newline check
echo -n "Checking for terminating newline... "
missing_newline=""
for file in $(git ls-files); do
  if [[ -f "${file}" ]]; then
    if grep -q "DO NOT EDIT" "${file}" 2>/dev/null; then
      continue
    fi
    if [[ -n "$(tail -c 1 "${file}" 2>/dev/null)" ]]; then
      missing_newline="${missing_newline}${file}\n"
    fi
  fi
done
if [[ -n "${missing_newline}" ]]; then
  echo "Failed. Files missing terminating newline:"
  echo -e "${missing_newline}"
  res=1
else
  echo "Passed."
fi

exit "${res}"
