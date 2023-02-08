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

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(git rev-parse --show-toplevel);
cd;

if [[ -f "${REPO_ROOT}/.bazelversion" ]]; then
export BAZEL_VERSION=$(cat "${REPO_ROOT}/.bazelversion");
echo "BAZEL_VERSION set to ${BAZEL_VERSION}";
else
export BAZEL_VERSION="5.3.0";
echo "BAZEL_VERSION - Falling back to 5.3.0";
fi;
/workspace/test-infra/images/kubekins-e2e/install-bazel.sh;
go install sigs.k8s.io/kubetest2@latest;
go install sigs.k8s.io/kubetest2/kubetest2-gce@latest;
go install sigs.k8s.io/kubetest2/kubetest2-tester-ginkgo@latest;
if [[ -f "${REPO_ROOT}/ginko-test-package-version.env" ]]; then
export TEST_PACKAGE_VERSION=$(cat "${REPO_ROOT}/ginko-test-package-version.env");
echo "TEST_PACKAGE_VERSION set to ${TEST_PACKAGE_VERSION}";
else
export TEST_PACKAGE_VERSION="v1.25.0";
echo "TEST_PACKAGE_VERSION - Falling back to v1.25.0";
fi;

${REPO_ROOT}/tools/update-kubernetes-version.sh

kubetest2 gce -v 2 --repo-root "${REPO_ROOT}" --build --up --down --test=ginkgo --node-size n1-standard-4 --master-size n1-standard-8 -- --test-package-version="${TEST_PACKAGE_VERSION}" --parallel=30 --test-args='--minStartupPods=8 --ginkgo.flakeAttempts=3' --skip-regex='\[Slow\]|\[Serial\]|\[Disruptive\]|\[Flaky\]|\[Feature:.+\]'