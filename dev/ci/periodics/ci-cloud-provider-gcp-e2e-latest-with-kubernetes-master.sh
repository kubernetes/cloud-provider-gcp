#!/bin/bash

# Wrapper script to run E2E tests using the kubernetes master branch via Make
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"
make test-e2e-latest-with-kubernetes-master
