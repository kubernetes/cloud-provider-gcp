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

KUBE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
source "${KUBE_ROOT}/tools/build/lib/common.sh"
source "${KUBE_ROOT}/tools/lib/util.sh"

RELEASE_STAGING_DIR="${OUT_DIR}/release-staging"
RELEASE_TARBALLS_DIR="${OUT_DIR}/release-tars"

KUBE_DOWNLOAD_DIR="${KUBE_DOWNLOAD_DIR:-"${OUT_DIR}/kube"}"

function build-server-tarball() {
	local tarball_name="kubernetes-server-linux-amd64.tar.gz"
	local staging_dir="${RELEASE_STAGING_DIR}/server-linux-amd64"

	rm -rf "${staging_dir}"
	mkdir -p "${staging_dir}"
	tar -C "${staging_dir}" -xf "${KUBE_DOWNLOAD_DIR}/${tarball_name}"
	for binary in "${SERVER_BINARIES[@]}"; do
		cp "${BIN_DIR}/linux-amd64/${binary}" "${staging_dir}/kubernetes/server/bin/"
	done
	# load cloud controller manager docker image and tag
	cp "${OUT_DIR}/images/cloud-controller-manager"{.tar,.docker_tag} "${staging_dir}/kubernetes/server/bin/"

	tar -C "${staging_dir}" -zcf "${RELEASE_TARBALLS_DIR}/${tarball_name}" kubernetes --owner=0 --group=0
}

function build-node-tarball() {
	local node_os="$1"
	local node_arch="$2"
	local node_platform="${node_os}-${node_arch}"
	local tarball_name="kubernetes-node-${node_platform}.tar.gz"
	local staging_dir="${RELEASE_STAGING_DIR}/node-${node_platform}"

	rm -rf "${staging_dir}"
	mkdir -p "${staging_dir}"
	tar -C "${staging_dir}" -xf "${KUBE_DOWNLOAD_DIR}/${tarball_name}"
	local exe_ext=""
	if [ "$node_os" == "windows" ]; then
		exe_ext=".exe"
	fi
	for binary in "${NODE_BINARIES[@]}"; do
		cp "${BIN_DIR}/${node_platform}/${binary}${exe_ext}" "${staging_dir}/kubernetes/node/bin/"
	done

	tar -C "${staging_dir}" -zcf "${RELEASE_TARBALLS_DIR}/${tarball_name}" kubernetes --owner=0 --group=0
}

function build-manifests-tarball() {
	local tarball_name="kubernetes-manifests.tar.gz"
	local staging_dir="${RELEASE_STAGING_DIR}/manifests"

	rm -rf "${staging_dir}"
	mkdir -p "${staging_dir}"
	tar -C "${staging_dir}" -xf "${KUBE_DOWNLOAD_DIR}/${tarball_name}"

	# add manifests
	cp "${KUBE_ROOT}/deploy/cloud-controller-manager.manifest" "${staging_dir}/kubernetes/gci-trusty/"

	# add addons
	mkdir -p "${staging_dir}/kubernetes/gci-trusty/cloud-controller-manager/"
	cp "${KUBE_ROOT}/deploy/"*.yaml "${staging_dir}/kubernetes/gci-trusty/cloud-controller-manager/"

	tar -C "${staging_dir}" -zcf "${RELEASE_TARBALLS_DIR}/${tarball_name}" kubernetes --owner=0 --group=0
}

function generate-tarball-checksums() {
	if which sha512sum >/dev/null 2>&1; then
		for tarball in \
			kubernetes-server-linux-amd64.tar.gz \
			kubernetes-node-linux-amd64.tar.gz \
			kubernetes-node-windows-amd64.tar.gz \
			kubernetes-manifests.tar.gz; do
			(cd "${RELEASE_TARBALLS_DIR}" && sha512sum "${tarball}" >"${tarball}.sha512")
		done
	fi
}

function build-release-tarballs() {
	KUBE_DOWNLOAD_DIR="${KUBE_DOWNLOAD_DIR}" "${KUBE_ROOT}/tools/build/download-kube-bin.sh"
	mkdir -p "${RELEASE_TARBALLS_DIR}"

	build-server-tarball &
	build-node-tarball linux amd64 &
	build-node-tarball windows amd64 &
	build-manifests-tarball &

	kube::util::wait-for-jobs || {
		echo "Tarball creation failed"
		exit 1
	}

	generate-tarball-checksums
}

build-release-tarballs
