#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset
set -x

KUBE_VERSION="1.14.0-beta.2"

KUBE_DEPS=(
 "k8s.io/api@kubernetes-${KUBE_VERSION}"
 "k8s.io/apiextensions-apiserver@kubernetes-${KUBE_VERSION}"
 "k8s.io/apimachinery@kubernetes-${KUBE_VERSION}"
 "k8s.io/apiserver@kubernetes-${KUBE_VERSION}"
 "k8s.io/client-go@48376054912de15b6386e4310192c4e8aab98403"
 "k8s.io/component-base@kubernetes-${KUBE_VERSION}"
 "k8s.io/kube-openapi@c59034cc13d587f5ef4e85ca0ade0c1866ae8e1d"
 "k8s.io/kubernetes@v${KUBE_VERSION}"
 "github.com/evanphx/json-patch@5858425f75500d40c52783dce87d085a483ce135"
)

main() {
  go get -d "${KUBE_DEPS[@]}"
  go mod tidy
}

main $@
