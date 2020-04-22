#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "${REPO_ROOT}"

source "${REPO_ROOT}"/test/boskos.sh

build(){
	bazel build //release:release-tars
}

up() {
	acquire_project
	trap "cleanup" EXIT
	"${REPO_ROOT}"/cluster/kube-up.sh
}

test(){
	kubectl get all --all-namespaces
}

down() {
	"${REPO_ROOT}"/cluster/kube-down.sh
}

cleanup(){
	STATUS=$?
	if [[ "${STATUS}" -ne 0 ]]; then
		echo "ERROR: kube-up exited with ${STATUS}"
	fi
	down
	cleanup_boskos
	exit "${STATUS}"
}

build
up
test
