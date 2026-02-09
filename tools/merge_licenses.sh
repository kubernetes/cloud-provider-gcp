#!/bin/bash

set -o xtrace
set -o errexit
set -o nounset
set -o pipefail

CURDIR=$(pwd)
output_path="${CURDIR}"/MERGED_LICENSES
# Truncate file.
echo >"${output_path}"

cat <<EOF >>"${output_path}"
------------------------------------------------------------
License for github.com/kubernetes/cloud-provider-gcp
------------------------------------------------------------
EOF
cat "${CURDIR}"/LICENSE >>"${output_path}"

cat <<EOF >>"${output_path}"
------------------------------------------------------------
Licenses of vendored software below
------------------------------------------------------------
EOF

for path in $(find "${CURDIR}"/vendor/ -name LICENSE); do
  >&2 echo "adding ${path}";
  # Trim working directory prefix from output.
  cat <<EOF >>"${output_path}"
------------------------------------------------------------
${path#"$CURDIR"}
------------------------------------------------------------
EOF
  cat "${path}" >>"${output_path}"
  echo >>"${output_path}"
done
