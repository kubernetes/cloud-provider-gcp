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

function build-binaries() {
	mkdir -p "${BIN_DIR}/linux-amd64" "${BIN_DIR}/windows-amd64"

	local goldflags goflags
	goldflags="${GOLDFLAGS=-s -w} $(kube::version::ldflags)"
	goflags=(
		-ldflags "${goldflags}"
		-trimpath
	)

	for binary in "${SERVER_BINARIES[@]}"; do
		# build for linux-amd64
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build "${goflags[@]}" -o "${BIN_DIR}/linux-amd64/${binary}"  "./cmd/${binary}"
	done

	for binary in "${NODE_BINARIES[@]}"; do
		# build for linux-amd64
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build "${goflags[@]}" -o "${BIN_DIR}/linux-amd64/${binary}" "./cmd/${binary}"
		# build for windows-amd64
		CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build "${goflags[@]}" -o "${BIN_DIR}/windows-amd64/${binary}.exe" "./cmd/${binary}"
	done
}

build-binaries
