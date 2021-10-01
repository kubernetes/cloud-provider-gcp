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

# Build Kubernetes release images. This will build the server target binaries,
# and create wrap them in Docker images, see `make release` for full releases

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${KUBE_ROOT}/build/lib/common.sh"

# Create a junit-style XML test report in this directory if set.
KUBE_JUNIT_REPORT_DIR=${KUBE_JUNIT_REPORT_DIR:-"${ARTIFACTS:-}"}

junitFilename() {
  if [[ -z "${KUBE_JUNIT_REPORT_DIR}" ]]; then
    echo ""
    return
  fi
  test_start_date=$(date "+%Y%m%d-%H%M%S")
  mkdir -p "${KUBE_JUNIT_REPORT_DIR}"
  echo "${KUBE_JUNIT_REPORT_DIR}/junit_${test_start_date}.xml"
}

GOTESTSUM="gotestsum"
if ! command -v "${GOTESTSUM}" >/dev/null 2>&1; then
  pushd "/" >/dev/null # move away so we can install tools
    go install gotest.tools/gotestsum@latest
    GOTESTSUM="${GOPATH}/bin/gotestsum"
  popd >/dev/null
fi

junit_args=()
junit_report_filename="$(junitFilename)"

if [ -n "${junit_report_filename}" ]; then
  junit_args=(--junitfile "${junit_report_filename}")
fi

"${GOTESTSUM}" --format standard-quiet "${junit_args[@]}" ./...
