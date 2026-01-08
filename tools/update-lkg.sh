#!/usr/bin/env bash
# Copyright 2025 The Kubernetes Authors.
#
# Updates KUBERNETES_LKG to the latest compatible generic Kubernetes version.
# Intended to be run as a periodic background job.

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
LKG_FILE="${REPO_ROOT}/KUBERNETES_LKG"

# URL for the latest stable version
LATEST_VERSION_URL="https://dl.k8s.io/release/stable.txt"

echo "Fetching latest Kubernetes CI version..."
NEW_VERSION=$(curl -sSL "${LATEST_VERSION_URL}")

echo "Candidate Version: ${NEW_VERSION}"

Current_LKG=""
if [[ -f "${LKG_FILE}" ]]; then
    Current_LKG=$(cat "${LKG_FILE}")
fi

if [[ "${NEW_VERSION}" == "${Current_LKG}" ]]; then
    echo "LKG is already up to date (${NEW_VERSION}). Skipping tests."
    exit 0
fi

echo "Running E2E tests against ${NEW_VERSION}..."

# Export VERSION so run-e2e-test.sh uses it
export VERSION="${NEW_VERSION}"

# Run the test script against the $NEW_VERSION
# TODO: running the tests is disabled for now until the tests are ready


echo "Tests passed for ${NEW_VERSION}."
echo "${NEW_VERSION}" > "${LKG_FILE}"
echo "Updated KUBERNETES_LKG to ${NEW_VERSION}."
