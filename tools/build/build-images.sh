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

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/../..

source "${KUBE_ROOT}/tools/build/lib/common.sh"
source "${KUBE_ROOT}/tools/version.sh"

function build-images() {
	get_version_vars
	local base_image=${BASE_IMAGE:-gcr.io/distroless/static}
	local image_registry=${IMAGE_REGISTRY:-gcr.io}
	local image_repo=${IMAGE_REPO:-k8s-image-staging}
	local image_tag=${IMAGE_TAG:-${KUBE_GIT_VERSION/+/-}}
	local image_root="${OUT_DIR}/images"
	local docker_stage="${OUT_DIR}/docker-stage"
	local bin_dir="${BIN_DIR}/linux-amd64"
	local docker_file_path="${KUBE_ROOT}/tools/build/server-image/Dockerfile"

	mkdir -p "${image_root}"

	for binary in "${SERVER_IMAGES[@]}"; do
		(
			rm -rf "${docker_stage}"
			mkdir -p "${docker_stage}"
			ln "${bin_dir}/${binary}" "${docker_stage}/${binary}"
			local image_full_tag="${image_registry}/${image_repo}/${binary}:${image_tag}"
			docker buildx build \
				-f "${docker_file_path}" \
				-t "${image_full_tag}" \
				--load \
				--build-arg BASEIMAGE="${base_image}" \
				--build-arg BINARY="${binary}" \
				"${docker_stage}"
			docker save "${image_full_tag}" -o "${image_root}/${binary}.tar"
			echo "${image_full_tag}" > "${image_root}/${binary}.docker_tag"
			rm -rf "${docker_stage}"
		)
	done
}

build-images
