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

DRY_RUN=${DRY_RUN:-true}
FORCE_BRANCH=${FORCE_BRANCH:-""}

# Get branch name
BRANCH_NAME=$(git rev-parse --abbrev-ref HEAD)

if [[ -n "$FORCE_BRANCH" ]]; then
    echo "Forcing branch name to: $FORCE_BRANCH"
    BRANCH_NAME="$FORCE_BRANCH"
fi

echo "Current branch: $BRANCH_NAME"

# Enforce release branch
if [[ ! "$BRANCH_NAME" =~ ^release-1\.([0-9]+)$ ]]; then
    echo "ERROR: This script must be run only in a release branch (release-1.X)."
    exit 1
fi

# Get version from go.mod
VERSION_STRING=$(grep -E 'k8s.io/client-go' go.mod | grep -oE 'v0\.[0-9]+\.[0-9]+' | head -n 1)
echo "Found client-go version string: $VERSION_STRING"

if [[ "$VERSION_STRING" =~ v0\.([0-9]+)\.([0-9]+) ]]; then
    MINOR="${BASH_REMATCH[1]}"
    PATCH="${BASH_REMATCH[2]}"
    echo "Extracted version parameters: MINOR=$MINOR, PATCH=$PATCH"
else
    echo "Could not determine client-go version from go.mod"
    exit 1
fi

TAG_PREFIX="v$MINOR.$PATCH."
echo "Tag prefix: $TAG_PREFIX"

# Fetch tags
echo "Fetching tags..."
git fetch --tags

# Find largest tag
TAGS=$(git tag -l "$TAG_PREFIX*")

LARGEST_CCM_PATCH=-1
for t in $TAGS; do
    if [[ "$t" =~ ^v$MINOR\.$PATCH\.([0-9]+)$ ]]; then
        p="${BASH_REMATCH[1]}"
        if (( p > LARGEST_CCM_PATCH )); then
            LARGEST_CCM_PATCH=$p
        fi
    fi
done

CCM_PATCH=$((LARGEST_CCM_PATCH + 1))
NEW_TAG="v$MINOR.$PATCH.$CCM_PATCH"

echo "Calculated new tag: $NEW_TAG"

if [ "$DRY_RUN" = "true" ]; then
    echo "DRY RUN: Skipping tag creation and push."
    echo "To create the tag, run with DRY_RUN=false"
    exit 0
fi

echo "Creating tag $NEW_TAG..."
git tag "$NEW_TAG" -m "CCM build for Kubernetes v1.$MINOR.$PATCH"

echo "Pushing tag $NEW_TAG..."
git push origin "$NEW_TAG"

echo "Successfully created and pushed tag $NEW_TAG"
