#!/usr/bin/env bash

# Copyright 2018 The Kubernetes Authors.
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

set -o xtrace
set -o errexit
set -o nounset
set -o pipefail

export MINOR_OLD=32 PATCH_OLD=2 MINOR=33 PATCH=0
export VERSION_FILES=(
  ginko-test-package-version.env
  tools/version.sh
  WORKSPACE
  )
export LIBRARY_FILES=(
    go.mod
    providers/go.mod
    test/e2e/go.mod
)

sed -i s/v1.$MINOR_OLD.$PATCH_OLD/v1.$MINOR.$PATCH/ "${VERSION_FILES[@]}"
sed -i s/v0.$MINOR_OLD.$PATCH_OLD/v0.$MINOR.$PATCH/ "${LIBRARY_FILES[@]}"
sed -i s/v1.$MINOR_OLD.$PATCH_OLD/v1.$MINOR.$PATCH/ "${LIBRARY_FILES[@]}"
for go_mod_file in "${LIBRARY_FILES[@]}"; do
  dir=$(dirname "$go_mod_file")
  pushd $dir
  echo "Tidying $dir"
  go mod tidy
  popd
done
sh ./tools/update_vendor.sh
