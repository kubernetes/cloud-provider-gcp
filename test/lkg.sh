#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

REPO_ROOT=$(git rev-parse --show-toplevel)
KUBETEST2_OPTIONS="gce -v 2 --repo-root=$REPO_ROOT --gcp-project=$(gcloud config get-value project) --build --up --down --test=ginkgo -- --parallel=30 --test-args='--minStartupPods=8' --skip-regex='\[Slow\]|\[Serial\]|\[Disruptive\]|\[Flaky\]|\[Feature:.+\]'"
KUBERNETES_REPO_ROOT="$REPO_ROOT/kubernetes-latest"
KUBERNETES_REPO_DIR="$KUBERNETES_REPO_ROOT/kubernetes"
cd $REPO_ROOT
mkdir -p $KUBERNETES_REPO_ROOT
cd $KUBERNETES_REPO_ROOT
git clone git@github.com:kubernetes/kubernetes.git
# prevent bazel from downloading the node/server tarballs so we can manually copy over the ones we want
cd $REPO_ROOT
patch defs/repo_rules.bzl << EOF
diff --git defs/repo_rules.bzl defs/repo_rules.bzl
index 74ac09d9..00d78b24 100644
--- defs/repo_rules.bzl
+++ defs/repo_rules.bzl
@@ -19,11 +19,6 @@ def _archive_url(folder, version, archive):
 def _fetch_kube_release(ctx):
     build_file_contents = BUILD_PRELUDE
     for archive in ctx.attr.archives:
-        ctx.download(
-            url = _archive_url(ctx.attr.folder, ctx.attr.version, archive),
-            output = archive,
-            sha256 = ctx.attr.archives[archive],
-        )
         build_file_contents += BUILD_TAR_TEMPLATE.format(
             paths.basename(archive).split(".")[0],
             archive,
EOF

cd $KUBERNETES_REPO_DIR && make quick-release && cd $REPO_ROOT
BAZEL_CACHE_DIR=$(bazel info output_base)/external/io_k8s_release/
# hack to get around bazel cleaning out the cache directory - intentionally run a build that will fail and be partially-cached
set +e
bazel build //release:release-tars
set -e
for KUBE_TARBALL in kubernetes-server-linux-amd64.tar.gz kubernetes-node-linux-amd64.tar.gz kubernetes-manifests.tar.gz
do
    cp $KUBERNETES_REPO_DIR/_output/release-tars/$KUBE_TARBALL $BAZEL_CACHE_DIR
done

kubetest2 $KUBETEST2_OPTIONS

KUBERNETES_LKG_FILE="$(git rev-parse --show-toplevel)/KUBERNETES_LKG_INFO"
cd $KUBERNETES_REPO_DIR
KUBERNETES_LKG_HASH=$(git rev-parse --short HEAD)
cd $REPO_ROOT
echo $KUBERNETES_LKG_HASH > $KUBERNETES_LKG_FILE
LKG_BRANCH="k8s-lkg-update-$KUBERNETES_LKG_HASH"
git checkout -b $LKG_BRANCH
git commit -m "Update kubernetes LKG version to $KUBERNETES_LKG_HASH"
git push origin $LKG_BRANCH