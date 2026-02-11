#!/bin/bash

# TODO: Use published release tars for cloud-provider-gcp if/once they exist
set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

REPO_ROOT=$GOPATH/src/k8s.io/cloud-provider-gcp
cd
export GO111MODULE=on
if [[ -f "${REPO_ROOT}/.bazelversion" ]]; then
  export BAZEL_VERSION=$(cat "${REPO_ROOT}/.bazelversion")
  echo "BAZEL_VERSION set to ${BAZEL_VERSION}"
else
  export BAZEL_VERSION="5.3.0"
  echo "BAZEL_VERSION - Falling back to 5.3.0"
fi
/workspace/test-infra/images/kubekins-e2e/install-bazel.sh
go install sigs.k8s.io/kubetest2@latest
go install sigs.k8s.io/kubetest2/kubetest2-gce@latest
go install sigs.k8s.io/kubetest2/kubetest2-tester-ginkgo@latest
if [[ -f "${REPO_ROOT}/ginko-test-package-version.env" ]]; then
  export TEST_PACKAGE_VERSION=$(cat "${REPO_ROOT}/ginko-test-package-version.env")
  echo "TEST_PACKAGE_VERSION set to ${TEST_PACKAGE_VERSION}"
else
  export TEST_PACKAGE_VERSION="v1.25.0"
  echo "TEST_PACKAGE_VERSION - Falling back to v1.25.0"
fi
kubetest2 gce -v 2 --repo-root $REPO_ROOT --build --up --down --test=ginkgo --master-size e2-standard-2  -- --test-package-version="${TEST_PACKAGE_VERSION}" --focus-regex='\[Conformance\]'
