#!/bin/bash

# Wrapper script to run conformance tests with GCEPD enabled via Make
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"
make test-conformance-latest-with-gcepd
