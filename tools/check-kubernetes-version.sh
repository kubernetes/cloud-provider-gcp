#!/usr/bin/env bash

# Copyright 2026 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${KUBE_ROOT}"

# Get branch name from argument or environment or git
BRANCH_NAME=${1:-${GITHUB_REF_NAME:-$(git rev-parse --abbrev-ref HEAD)}}

echo "Checking branch: $BRANCH_NAME"

# Check if branch matches release-1.X
if [[ ! "$BRANCH_NAME" =~ ^release-1\.([0-9]+)$ ]]; then
    echo "Not a release branch (release-1.X), skipping check."
    exit 0
fi

EXPECTED_MINOR="${BASH_REMATCH[1]}"
echo "Expected minor version: $EXPECTED_MINOR"

# Get version from go.mod
VERSION_STRING=$(grep -E 'k8s.io/client-go' go.mod | grep -oE 'v0\.[0-9]+\.[0-9]+' | head -n 1)
echo "Found client-go version string: $VERSION_STRING"

if [[ "$VERSION_STRING" =~ v0\.([0-9]+)\. ]]; then
    ACTUAL_MINOR="${BASH_REMATCH[1]}"
    echo "Actual client-go minor version: $ACTUAL_MINOR"
else
    echo "Could not determine client-go version from go.mod"
    exit 1
fi

if [ "$EXPECTED_MINOR" != "$ACTUAL_MINOR" ]; then
    echo "ERROR: Branch version (1.$EXPECTED_MINOR) does not match client-go version (v0.$ACTUAL_MINOR)"
    exit 1
fi

echo "Success: Branch version matches client-go version."
exit 0
