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
	# TODO(jhf) remove smoke tests once we have proper tests.
	kubectl create deploy nginx --image=mirror.gcr.io/library/nginx:latest
	kubectl expose deploy/nginx --port=80 --target-port=80 --type=LoadBalancer
	external_ip=""
	while true; do
		external_ip=$(kubectl get svc nginx -o template --template='{{range .status.loadBalancer.ingress}}{{.ip}}{{end}}')
		if [ -n "$external_ip" ]; then
			break
		fi
		sleep 1
	done
	curl -o/dev/null --quiet "http://$external_ip"
}

dumplogs(){
        mkdir -p "${ARTIFACTS}"/cluster-logs
        kubectl cluster-info dump > "${ARTIFACTS}"/cluster-logs/cluster-info.log
        KUBE_GCE_INSTANCE_PREFIX="${KUBE_GCE_INSTANCE_PREFIX:-kubernetes}" "${REPO_ROOT}"/cluster/log-dump/log-dump.sh "${ARTIFACTS}"/cluster-logs
}

down() {
	"${REPO_ROOT}"/cluster/kube-down.sh
}

cleanup(){
	STATUS=$?
	if [[ "${STATUS}" -ne 0 ]]; then
          echo "ERROR: "${FUNCNAME[1]}"() exited with ${STATUS}"
	fi
        dumplogs || true
	down
	cleanup_boskos
	exit "${STATUS}"
}

build
up
test
