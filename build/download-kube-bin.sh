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

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${KUBE_ROOT}/build/lib/common.sh"

KUBE_VERSION=v1.22.0

KUBERNETES_RELEASE_URL="${KUBERNETES_RELEASE_URL:-https://dl.k8s.io}"
DOWNLOAD_URL_PREFIX="${KUBERNETES_RELEASE_URL}/${KUBE_VERSION}"

KUBE_DOWNLOAD_DIR="${KUBE_DOWNLOAD_DIR:-"${OUT_DIR}/kube"}"

# checksums can be found at
# https://dl.k8s.io/${KUBE_VERSION}/SHA512SUMS
# e.g. https://dl.k8s.io/v1.21.0/SHA512SUMS
# or append .sha256 to the url for an individual checksum

KUBE_TARBALLS=(
  kubernetes-server-linux-amd64.tar.gz:d54435de50214faabc49e3659625a689623508128ca9a4f97b4f2c54b40bc9e14dd17e1971c06c90aa74fc335d0038a7ac4b7b90882edb0944af99354d6c9762
  kubernetes-node-linux-amd64.tar.gz:aa990405a1c6bd6737a8ff89fd536ba28ad62dec7de2e44ae223f4fcb42d6a9ffdfb324144def946b777ac7ba6fac085a49a7977cb79289a3256cced783bf215
  kubernetes-node-windows-amd64.tar.gz:9cc73fb1d3f9ec926fd09bc3904d62ec79da4a3c4fb9a5c4c784bc1f08c650711c21fb30874b05db4bd354e4d04b0153296180d89a53c04d9241dd6a1384510d
  kubernetes-manifests.tar.gz:b531f48757c94d211cd097387a187e892c6ab794317a43adfe378524f0d479a0cff98becf5a0a570f244ebc63355df893c01fd2d57836d939e8fd82b392ba259
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
