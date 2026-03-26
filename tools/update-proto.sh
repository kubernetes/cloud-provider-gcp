#!/usr/bin/env bash

# Copyright 2026 The Kubernetes Authors.

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

set -o xtrace
set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${KUBE_ROOT}"
TOOLS_BIN="${PWD}/_tmp/bin"
mkdir -p "${TOOLS_BIN}"
export PATH="${TOOLS_BIN}:${PATH}"

# Install protoc if not present
if ! command -v protoc &> /dev/null; then
  echo "protoc not found, installing locally..."
  PROTOC_VERSION="34.1"
  PROTOC_ZIP="protoc-${PROTOC_VERSION}-linux-x86_64.zip"
  PROTOC_URL="https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/${PROTOC_ZIP}"
  
  TMP_PROTOC_DIR=$(mktemp -d "${PWD}/_tmp/protoc.XXXXXX")
  
  curl -L -o "${TMP_PROTOC_DIR}/${PROTOC_ZIP}" "${PROTOC_URL}"
  unzip -q -o "${TMP_PROTOC_DIR}/${PROTOC_ZIP}" -d "${TMP_PROTOC_DIR}"
  
  mv "${TMP_PROTOC_DIR}/bin/protoc" "${TOOLS_BIN}/"
  chmod +x "${TOOLS_BIN}/protoc"
  
  rm -rf "${TMP_PROTOC_DIR}"
fi

# Install protoc-gen-go and protoc-gen-go-grpc if not present
if ! command -v protoc-gen-go &> /dev/null || ! command -v protoc-gen-go-grpc &> /dev/null; then
  echo "protoc-gen-go/grpc not found, installing..."
  export GOBIN="${TOOLS_BIN}"
  
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Find all proto files outside of vendor and temporary/artifact directories
proto_files=$(find . -name "*.proto" -not -path "./vendor/*" -not -path "./_*" )

if [[ -z "$proto_files" ]]; then
  echo "No proto files found to update."
  exit 0
fi

for proto in $proto_files; do
  echo "Generating code for $proto..."
  # Strip leading ./ for better logging if needed
  clean_proto="${proto#./}"
  
  protoc --go_out=. --go_opt=paths=source_relative \
         --go-grpc_out=. --go-grpc_opt=paths=source_relative \
         --proto_path=. "$clean_proto"
done

echo "Proto files updated successfully!"
