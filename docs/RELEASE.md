# Release Instructions

## Version Schema
Cloud controller manager (CCM) has unique image builds for every Kubernetes minor 
version.

**Format**: K8S_MINOR.K8S_PATCH.CCM_PATCH (e.g. 31.4.0)

* **K8S_MINOR**: Represents the Kubernetes minor version number. For Kubernetes
version 1.31, K8S_MINOR would be 31.
* **K8S_PATCH**: The kubernetes patch version used in the current release.
* **CCM_PATCH**: The patch version for CCM, starting at 0 and incremented for
each cherry-picked change applied to a previously released version.

See k8s release [schedule](https://kubernetes.io/releases/).

CCM releases are made on release branches. The repository maintains a dedicated release branch for each minor version of Kubernetes.

## Update the Library Version with k/k Release
Update library versions in each release branch, including master, corresponding to the Kubernetes minor versions under maintenance (a total of three Kubernetes minor versions).

1. Set common variables. Many of the following commands expect release-specific variables to be set. Set them before continuing, and set them again when resuming.
```bash
MINOR_OLD=31 PATCH_OLD=0 MINOR=32 PATCH=0 # Set appropriately for the release for kubernetes versions
VERSION_FILES=(
  ginko-test-package-version.env
  tools/version.sh
  WORKSPACE
  )
LIBRARY_FILES=(
    go.mod
    providers/go.mod
    test/e2e/go.mod
)
```
2. Bump library versions.
```bash
sed -i s/v1.$MINOR_OLD.$PATCH_OLD/v1.$MINOR.$PATCH/ "${VERSION_FILES[@]}"
for f in "${LIBRARY_FILES[@]}" go.work; do
  sed -E -i \
    "s#(k8s\\.io/[^[:space:]]+[[:space:]]+v0\\.)$MINOR_OLD\\.$PATCH_OLD#\\1$MINOR.$PATCH#g; s#(k8s\\.io/kubernetes[[:space:]]+v1\\.)$MINOR_OLD\\.$PATCH_OLD#\\1$MINOR.$PATCH#g" \
    "$f"
done
for go_mod_file in "${LIBRARY_FILES[@]}"; do
  dir=$(dirname "$go_mod_file")
  pushd $dir
  echo "Tidying $dir"
  go mod tidy
  popd
done
make update-vendor
```
3. In [WORKSPACE](https://github.com/kubernetes/cloud-provider-gcp/blob/master/WORKSPACE), update `fetch_kube_release` sha to the desired release version.
    * Note: The current Kubernetes release is using sha512 hash while cloud-provider-gcp is using sha256. Re-sha with command `sha256sum` if needed. Use this command to generate values automatically.
```bash
export KUBE_VERSION=v1.$MINOR.$PATCH
tools/sha256_generator.sh
```

### Testing
1. Run `make verify`.
1. Bring the cluster up with `make test-cluster-up`
1. Run conformance tests locally with `make test-cluster-e2e-test`

## Create Release Branch
This is only necessary for CCM releases corresponding to a Kubernetes minor version release. Create a branch from the latest commit on master with the dependency updates mentioned above.

 **_NOTE:_**  Only members in the OWNERS file have the permission to create a branch.

## Tagging for New Cloud-controller-manager Versions

To trigger a new image for cloud-controller-manager, you need to add a git tag on the corresponding release branch.
This needs to have the format `vX.Y.Z`. For example.

```
git tag -a v35.0.5 -m "CCM build for Kubernetes v35.0.5"
```

The major version X corresponds to the Kubernetes minor version. The minor
version Y corresponds to the Kubernetes patch version and the patch version Z
corresponds to the CCM patch version.

**_NOTE:_**  Only members in the OWNERS file have the permission to push a tag.

## Publish OSS Images
[Postsubmit](https://github.com/kubernetes/test-infra/blob/a96c5d60b64b09b5b5f7817745477d0af3122aa1/config/jobs/image-pushing/k8s-staging-cloud-provider-gcp.yaml#L62-L91) job automatically pushes OSS images  via [cloud build](https://github.com/kubernetes/cloud-provider-gcp/blob/master/cloudbuild.yaml).
Verify job excecutions in [testgrid](https://testgrid.k8s.io/provider-gcp-postsubmits#post-cloud-provider-gcp-push-images). See example [execution](https://prow.k8s.io/view/gs/kubernetes-ci-logs/logs/post-cloud-provider-gcp-push-images/1894307075022917632). Image is uploaded to staging bucket [gcr.io/k8s-staging-cloud-provider-gcp](https://console.cloud.google.com/gcr/images/k8s-staging-cloud-provider-gcp/GLOBAL/cloud-controller-manager). Make sure the latest image with the correponsing tag `vK8S_MINOR.K8S_PATCH.CCM_PATCH`
shows up in the bucket.


### Promote Image
Once the image is pushed to the above staging bucket, create a PR in the [kubernetes/k8s.io](https://github.com/kubernetes/k8s.io) repository to promote the image to the official Kubernetes [registry](https://registry.k8s.io). It is recommended to use the [kpromo](https://github.com/kubernetes-sigs/promo-tools/blob/main/docs/promotion-pull-requests.md#promoting-images) tool to create the PR automatically. See example PR [here](https://github.com/kubernetes/k8s.io/pull/7848).

To verify the image is uploaded to the Kubernetes registry, run:
```bash
docker pull registry.k8s.io/cloud-provider-gcp/cloud-controller-manager:v$K8S_MINOR.$K8S_PATCH.$CCM_PATCH
```

### Update GCE Job Version
Update the GCE job [version](https://github.com/kubernetes/kubernetes/blob/1b4c3483cea4aae55d2eb815a0ff855b587c9a67/cluster/gce/manifests/cloud-controller-manager.manifest#L26) in the `kubernetes/kubernetes` repository with the latest CCM image version. This update is used to run the Kubernetes E2E tests.

## GKE release
Follow instructions at go/gke-ccm-releasing.
