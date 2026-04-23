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

# This script helps create GitHub releases from existing tags.
# Usage: ./tools/create-github-release.sh <tag> [draft]

set -euo pipefail

REPO="kubernetes/cloud-provider-gcp"

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo "Error: gh CLI is not installed. Please install it first:"
    echo "  See: https://cli.github.com/"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo "Error: Not authenticated with GitHub. Please run 'gh auth login' first."
    exit 1
fi

# Check arguments
if [ $# -lt 1 ]; then
    echo "Usage: $0 <tag> [draft]"
    echo ""
    echo "Arguments:"
    echo "  tag   - The git tag to create a release from (e.g., v35.0.8)"
    echo "  draft - Optional. If set to 'true', creates a draft release"
    echo ""
    echo "Examples:"
    echo "  $0 v35.0.8           # Create a published release for tag v35.0.8"
    echo "  $0 v35.0.8 true      # Create a draft release for tag v35.0.8"
    exit 1
fi

TAG="$1"
DRAFT="${2:-false}"

# Verify tag exists
if ! git rev-parse "$TAG" &> /dev/null; then
    echo "Error: Tag '$TAG' does not exist locally."
    echo "Please fetch tags first: git fetch --tags"
    exit 1
fi

# Generate release notes
echo "Generating release notes for $TAG..."

# Get the previous tag for comparison
PREV_TAG=$(git tag -l 'v*' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | grep -B1 "^$TAG$" | head -1)

if [ -n "$PREV_TAG" ] && [ "$PREV_TAG" != "$TAG" ]; then
    echo "Comparing with previous tag: $PREV_TAG"
    CHANGES=$(git log "$PREV_TAG..$TAG" --pretty=format:"- %s" --no-merges 2>/dev/null || echo "No changes found")
else
    echo "No previous tag found for comparison"
    CHANGES=$(git log "$TAG" --pretty=format:"- %s" --no-merges -10 2>/dev/null || echo "No changes found")
fi

# Get tag message if annotated
TAG_MESSAGE=$(git tag -l --format='%(contents)' "$TAG" 2>/dev/null | head -5 || echo "")

# Create release notes
RELEASE_NOTES="## Release $TAG

"

if [ -n "$TAG_MESSAGE" ]; then
    RELEASE_NOTES+="$TAG_MESSAGE

"
fi

RELEASE_NOTES+="### Changes

$CHANGES

### Artifacts

Release artifacts are available on the [releases page](https://github.com/$REPO/releases/tag/$TAG).

For container images, see the [container registry](https://console.cloud.google.com/gcr/images/k8s-staging-cloud-provider-gcp).

---

**Full Changelog**: https://github.com/$REPO/compare/${PREV_TAG:-$TAG}...$TAG
"

# Save release notes to temp file
NOTES_FILE=$(mktemp)
echo "$RELEASE_NOTES" > "$NOTES_FILE"

echo ""
echo "Release notes preview:"
echo "---"
cat "$NOTES_FILE"
echo "---"
echo ""

# Confirm before proceeding
read -p "Create release for $TAG? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    rm "$NOTES_FILE"
    exit 1
fi

# Create the release
echo "Creating release for $TAG..."

DRAFT_FLAG=""
if [ "$DRAFT" = "true" ]; then
    DRAFT_FLAG="--draft"
    echo "Creating as draft release..."
fi

gh release create "$TAG" \
    --repo "$REPO" \
    --title "Release $TAG" \
    --notes-file "$NOTES_FILE" \
    $DRAFT_FLAG

RELEASE_URL="https://github.com/$REPO/releases/tag/$TAG"
echo ""
echo "Release created successfully!"
echo "URL: $RELEASE_URL"

rm "$NOTES_FILE"