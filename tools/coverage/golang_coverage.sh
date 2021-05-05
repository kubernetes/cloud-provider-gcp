#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
set -x

PKG_ROOT=$1
testdirs() {
  find -L "${PKG_ROOT}" \
    -name \*_test.go -print0 | xargs -0n1 dirname | \
    sed "s|^${PKG_ROOT}/|./|" | LC_ALL=C sort -u
}

coverprofile="${PKG_ROOT}"/coverage.out
# Remove any old cover profile so that the run is clean.
rm -f "${coverprofile}"

echo "Verifying coverage"
go test -coverprofile="${coverprofile}" $(testdirs) -mod=readonly
