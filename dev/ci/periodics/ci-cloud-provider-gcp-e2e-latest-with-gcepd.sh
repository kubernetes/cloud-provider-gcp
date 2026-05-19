#!/bin/bash

# Wrapper script to run E2E tests with GCEPD enabled via Make
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"
make test-e2e-latest-with-gcepd
