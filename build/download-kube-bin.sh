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

KUBE_VERSION=v1.23.3

KUBERNETES_RELEASE_URL="${KUBERNETES_RELEASE_URL:-https://dl.k8s.io}"
DOWNLOAD_URL_PREFIX="${KUBERNETES_RELEASE_URL}/${KUBE_VERSION}"

KUBE_DOWNLOAD_DIR="${KUBE_DOWNLOAD_DIR:-"${OUT_DIR}/kube"}"

# checksums can be found at
# https://dl.k8s.io/${KUBE_VERSION}/SHA512SUMS
# e.g. https://dl.k8s.io/v1.21.0/SHA512SUMS
# or append .sha256 to the url for an individual checksum

KUBE_TARBALLS=(
  kubernetes-server-linux-amd64.tar.gz:667bc04778070685e5fb5b6281fe78263c5081af0613adfe9a68df0695210cb2273e89a1d37a27e4cbf947b9e565ef7697d8b90ddfba23aeeb4c9f8474a373c5
  kubernetes-node-linux-amd64.tar.gz:9fd17ed04dc8e13ba5b4d67ec657b8afba721c344bd9785669af3def481dcbd8a2eecb02e54e5eebd0559645c6e819f757c49de731e53073f06a12d871e569eb
  kubernetes-node-windows-amd64.tar.gz:8d687018bf4b70065d4871406702d57f0ef14abb6c8e8bd7635d2d94f8a56aead9a641ede4477e34534bc705e76bb94cec10dbb9414c5885ad0a5d07d1105401
  kubernetes-manifests.tar.gz:92a84cb49e24390ad65b0c411aaf38b3fbc85c868298096351cf84d65b62057be45c2e46ed6d6456d84e267ee1d1617fcd413520964fc32048c21238ec3d8c6c
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
