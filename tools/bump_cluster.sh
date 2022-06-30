#!/usr/bin/env bash

# Copyright 2022 The Kubernetes Authors.
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

CLOUD_PROVIDER_PATH="${CLOUD_PROVIDER_PATH:-$GOPATH/src/k8s.io/cloud-provider-gcp}"
KUBERNETES_PATH="${KUBERNETES_PATH:-$GOPATH/src/k8s.io/kubernetes}"

cd "${CLOUD_PROVIDER_PATH}"

rm -rf cluster
cp -R "${KUBERNETES_PATH}/cluster" ./

rm -rf cluster/images
rm -rf cluster/README.md
git checkout cluster/addons/pdcsi-driver
git checkout cluster/OWNERS
git checkout cluster/addons/cloud-controller-manager
git checkout cluster/bin/kubectl
git checkout cluster/clientbin.sh
git checkout cluster/common.sh
git checkout cluster/gce/manifests/cloud-controller-manager.manifest
git checkout cluster/gce/manifests/pdcsi-controller.yaml
git checkout cluster/kubectl.sh
git checkout cluster/util.sh

for f in $(git ls-files -m | grep -e '\.sh$'); do gawk -i inplace '{ gsub(/source "\$\{KUBE_ROOT\}\/hack\/lib\/util\.sh"$/, "source \"${KUBE_ROOT}/cluster/util.sh\"") }; { print }' $f ; done

for new_owner_file in $(git ls-files --others --exclude-standard | grep -e 'OWNERS$'); do
  rm $new_owner_file
done

for build_file in $(git ls-files -d | grep BUILD); do
  git checkout $build_file
done

echo "

Please review all remaning chagnes. We want to keep:
  * Custom kubectl
  * PDSCI plugin
  * Enabled CCM
  * Credential provider specific code (look for credential sidecar)
  * Bumped master size
  * Bumped node size
  * Enabled ENABLE_DEFAULT_STORAGE_CLASS
  * Restore 'cluster/util.sh' instead of 'hack/lib/util.sh'
 
 This might not be the whole list of changes that needs to be reverted.

"
