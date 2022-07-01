#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset
set -x

KUBE_VERSION=${KUBE_VERSION:-"24.2"}

KUBE_DEPS=(
 "k8s.io/api@v0.${KUBE_VERSION}"
 "k8s.io/apiextensions-apiserver@v0.${KUBE_VERSION}"
 "k8s.io/apimachinery@v0.${KUBE_VERSION}"
 "k8s.io/apiserver@v0.${KUBE_VERSION}"
 "k8s.io/client-go@v0.${KUBE_VERSION}"
 "k8s.io/code-generator@v0.${KUBE_VERSION}"
 "k8s.io/component-base@v0.${KUBE_VERSION}"
 "k8s.io/component-helpers@v0.${KUBE_VERSION}"
 "k8s.io/controller-manager@v0.${KUBE_VERSION}"
 "k8s.io/kube-controller-manager@v0.${KUBE_VERSION}"
 "k8s.io/kubelet@v0.${KUBE_VERSION}"
#  "k8s.io/kubernetes@v1.${KUBE_VERSION}"
 "k8s.io/metrics@v0.${KUBE_VERSION}"
)

KUBE_DEPS_PROVIDERS=(
 "k8s.io/api@v0.${KUBE_VERSION}"
 "k8s.io/apimachinery@v0.${KUBE_VERSION}"
 "k8s.io/client-go@v0.${KUBE_VERSION}"
 "k8s.io/component-base@v0.${KUBE_VERSION}"
)

main() {
  go get -d "${KUBE_DEPS[@]}"
  go mod tidy
  cd providers
  go get -d "${KUBE_DEPS_PROVIDERS[@]}"
  go mod tidy
}

main $@
