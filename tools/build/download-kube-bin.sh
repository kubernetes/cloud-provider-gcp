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

KUBE_VERSION=v1.21.0

KUBERNETES_RELEASE_URL="${KUBERNETES_RELEASE_URL:-https://dl.k8s.io}"
DOWNLOAD_URL_PREFIX="${KUBERNETES_RELEASE_URL}/${KUBE_VERSION}"

KUBE_DOWNLOAD_DIR="${KUBE_DOWNLOAD_DIR:-"${OUT_DIR}/kube"}"

# checksums can be found at
# https://dl.k8s.io/${KUBE_VERSION}/SHA512SUMS
# e.g. https://dl.k8s.io/v1.21.0/SHA512SUMS
# or append .sha256 to the url for an individual checksum

KUBE_TARBALLS=(
  kubernetes-server-linux-amd64.tar.gz:3941dcc2309ac19ec185603a79f5a086d8a198f98c04efa23f15a177e5e1f34946ea9392ba9f5d24d0d727839438f067fef1001fc6e88b27b8b01e35bbd962ca
  kubernetes-node-linux-amd64.tar.gz:c1831c708109c31b3878e5a9327ea4b9e546504d0b6b00f3d43db78b5dd7d5114d32ac24a9a505f9cadbe61521f0419933348d2cd309ed8cfe3987d9ca8a7e2c
  kubernetes-node-windows-amd64.tar.gz:b82e94663d330cff7a117f99a7544f27d0bc92b36b5a283b3c23725d5b33e6f15e0ebf784627638f22f2d58c58c0c2b618ddfd226a64ae779693a0861475d355
  kubernetes-manifests.tar.gz:f2d96007d71cbcf929978208d33c498713771c45a84b5c55c9673dd0e601370d88c2e5ec5a7d7e4618b422a240dcb31133e0eec25a2d9374a790ab8d24ab9a87
)

function download_tarball() {
  local -r download_path="$1"
  local -r file="$2"

  url="${DOWNLOAD_URL_PREFIX}/${file}"
  mkdir -p "${download_path}"

  if [[ $(which gsutil) ]] && [[ "$url" =~ ^https://storage.googleapis.com/.* ]]; then
    gsutil cp "${url//'https://storage.googleapis.com/'/'gs://'}" "${download_path}/${file}"
  elif [[ $(which curl) ]]; then
    # if the url belongs to GCS API we should use oauth2_token in the headers
    curl_headers=""
    if { [[ "${KUBERNETES_PROVIDER:-gce}" == "gce" ]] || [[ "${KUBERNETES_PROVIDER}" == "gke" ]]; } &&
      [[ "$url" =~ ^https://storage.googleapis.com.* ]]; then
      curl_headers="Authorization: Bearer $(gcloud auth print-access-token)"
    fi
    curl ${curl_headers:+-H "${curl_headers}"} -fL --retry 3 --keepalive-time 2 "${url}" -o "${download_path}/${file}"
  elif [[ $(which wget) ]]; then
    wget "${url}" -O "${download_path}/${file}"
  else
    echo "Couldn't find gsutil, curl, or wget.  Bailing out." >&2
    exit 4
  fi
  echo
}

function download-kube-binaries() {
  mkdir -p "${KUBE_DOWNLOAD_DIR}"

  for item in "${KUBE_TARBALLS[@]}"; do
    tarball="${item%%:*}"
    sha512="${item#*:}"
    dest="${KUBE_DOWNLOAD_DIR}/${tarball}"
    if ! validate-sha512 "$dest" "$sha512"; then
      echo downloading "${tarball}"
      download_tarball "${KUBE_DOWNLOAD_DIR}" "${tarball}"
      validate-sha512 "$dest" "$sha512" || exit 1
    fi
  done
}

function validate-sha512() {
  local -r file="$1"
  local -r expected_sha512="$2"
  if [ ! -f "$file" ]; then
    echo "$file" does not exist
    return 1
  fi
  actual_sha512=$(sha512sum "${file}" | awk '{ print $1 }') || true
  if [[ "${actual_sha512}" != "${expected_sha512}" ]]; then
    echo "${file} corrupted, ssha512 ${actual_sha512} doesn't match expected ${expected_sha512}"
    return 1
  fi
}

download-kube-binaries
