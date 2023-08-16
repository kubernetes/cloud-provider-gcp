# Update cloud-provider-gcp with k/k release

Manual instruction how to update `cloud-provider-gcp` repository.

## Workflow

1. Update library to the desired version.
    * [ginko-test-package-version.env](https://github.com/kubernetes/cloud-provider-gcp/blob/master/ginko-test-package-version.env), [go.mod](https://github.com/kubernetes/cloud-provider-gcp/blob/master/go.mod), [providers/go.mod](https://github.com/kubernetes/cloud-provider-gcp/blob/master/providers/go.mod), [crd/go.mod](https://github.com/kubernetes/cloud-provider-gcp/blob/master/crd/go.mod) and [crd/hack/go.mod](https://github.com/kubernetes/cloud-provider-gcp/blob/master/crd/hack/go.mod) describe the required libraries. Update the version of each dependency to the desired Kubernetes release version. Run `go mod tidy` after update. First in `crd/hack`, then in `crd` and `/providers`, then in a root path.
    * Run `tools/update_vendor.sh`
1. In [WORKSPACE](https://github.com/kubernetes/cloud-provider-gcp/blob/master/WORKSPACE), update `fetch_kube_release` sha and version to the desired release version.
    * Note: The current Kubernetes release is using sha512 hash while cloud-provider-gcp is using sha256. Re-sha with command `sha256sum` if needed. Use `export KUBE_VERSION=v1.X.Y; tools/sha256_generator.sh` to generate values automatically.
1. Update `KUBE_GIT_VERSION `in `https://github.com/kubernetes/cloud-provider-gcp/blob/9f5cdad672954777791e722baa607ee2a3912002/tools/version.sh#L77` with the right tag.
1. Update `/cluster` directory if needed. Script under `/cluster` is used to provision a k8s cluster on GCE using [kube-up.sh](https://github.com/kubernetes/cloud-provider-gcp/blob/master/cluster/kube-up.sh)
    1. Rebase /cluster directory with the /cluster directory from kubernetes/kubernetes at desired Kubernetes release version. (kubernetes/kubernetes/cluster/images should not be pulled in cloud-provide-gcp.)
    1. Selectively re-applies direct contributions made to the /cluster directory of cloud-provider-gcp that are clobbered by the rebase of the /cluster directory. (see reference in the end of this documentation)
    1. Remove any changes regarding OWNERS files.
    * Note: Use `tools/bump_cluster.sh` to automate part of this process.
1. Testing:
    1. Run `tools/verify-all.sh`.
    1. Build `cloud-provider-gcp` with command `bazel clean && bazel build //release:release-tars`.
    1. Bring the cluster up with `kubetest2 gce -v 2 --repo-root $REPO\_ROOT --build --up`
    1. Run conformance tests locally with `kubetest2 gce -v 2 --repo-root $REPO\_ROOT --build --up --down --test=ginkgo -- --test-package-version=[your version] --focus-regex='\[Conformance\]'`
    1. Note: if kubetest2 not working as expected, try with:
    ```
    go get sigs.k8s.io/kubetest2@latest
    go get sigs.k8s.io/kubetest2/kubetest2-gce@latest;
    go get sigs.k8s.io/kubetest2/kubetest2-tester-exec@latest;
    go get sigs.k8s.io/kubetest2/kubetest2-tester-ginkgo@latest;
    ```

## Reference of cloud-provider-gcp specific changes in /cluster directory

*   [Deploy Kubernetes from cloud-provider-gcp. #143](https://github.com/kubernetes/cloud-provider-gcp/pull/143)
    *   **cluster/addons/addon-manager/kube-addons.sh (moved mostly to cluster/addons/addon-manager/kube-addons-main.sh)**
        *   **is\_cloud\_leader decl and if is\_leader || is\_cloud\_leader**
    *   **cluster/common.sh**
        *   **hack/lib/util.sh -> cluster/util.sh**
        *   **set\_binary\_version locations changes to just bazel-bin**
        *   **verify-kube-binaries hack to get a local kubectl from existing tars**
    *   **cluster/gce/config-test.sh**
        *   **CLOUD\_CONTROLLER\_MANAGER\_TEST\_ARGS**
    *   **cluster/gce/gci/configure-helper.sh**
        *   **CLOUD\_CONTROLLER\_MANAGER\_TOKEN set and used to append\_or\_replace\_prefixed\_line**
        *   **system:cloud-controller-manager added (2 times) in Policy objects**
        *   **start-kube-controller-manager: --cloud-provider=external**
        *   **start-cloud-controller-manager decl**
        *   **CLOUD\_CONTROLLER\_MANAGER\_CPU\_REQUEST**
        *   **CLOUD\_CONTROLLER\_MANAGER\_TOKEN**
        *   **start-cloud-controller-manager called in main**
*   **cluster/gce/gci/configure.sh**
    *   **try-load-docker-image "${img\_dir}/cloud-controller-manager.tar"**
    *   **cp "${src\_dir}/cloud-controller-manager.tar" "${dst\_dir}"**
*   [Add basic cluster up/down e2e test. #144](https://github.com/kubernetes/cloud-provider-gcp/pull/144)
    *   **cluster/gce/gci/configure.sh - sha512 changes**
*   [Add logdump for e2e create. #148](https://github.com/kubernetes/cloud-provider-gcp/pull/148)
    *   **cluster/log-dump/log-dump.sh - adds cloud-controller-manager.log as party of cherry-pick**
*   [Fix CCM image. #151](https://github.com/kubernetes/cloud-provider-gcp/pull/151)
    *   **cloud-node-controller-role.yaml fixes**
    *   **cluster/gce/gci/configure-helper.sh**
        *   **add convert-manifest-params**
        *   **start-kube-controller-manager**
            *   **--external-cloud-volume-plugin=gce**
        *   **start-cloud-controller-manager**
            *   **--v=4, params+=" --port=10253"**
        *   **kube\_rc\_docker\_tag changes**
    *   **cluster/gce/gci/configure-kubeapiserver.sh**
        *   **start-kube-apiserver: --cloud-provider=external**
    *   **cluster/gce/manifests/cloud-controller-manager.manifest**
        *   **--log-file, --logtostderr (switched to --log\_file by ???)**
*   [Fix shellcheck failure in cluster/gce/config-default.sh #152](https://github.com/kubernetes/cloud-provider-gcp/pull/152)
    *   **cherry-pick of shellcheck (**[Fix shellcheck failure in cluster/gce/config-default.sh kubernetes#82062](https://github.com/kubernetes/kubernetes/pull/82062)**) - This is a no-op, just mentioned here for completeness.**
*   [Create the bucket for tars based on $PROJECT #154](https://github.com/kubernetes/cloud-provider-gcp/pull/154)
    *   **cluster/gce/util.sh: add PROJECT to gsutil call (also in upstream ?)**
*   [Add auth-provider-gcp for out-of-tree credential provision #168](https://github.com/kubernetes/cloud-provider-gcp/pull/168)
    *   **cluster/gce/config-default.sh: ENABLE\_CREDENTIAL\_SIDECAR setup**
    *   **cluster/gce/gci/configure-helper.sh: create-sidecar-config if ENABLE\_CREDENTIAL\_SIDECAR and create-sidecar-config decl**
    *   **cluster/gce/gci/configure.sh: mv auth-provider-gcp to bin**
    *   **cluster/gce/util.sh: construct-linux-kubelet-flags with --image-credential-provider-config, ENABLE\_CREDENTIAL\_SIDECAR: $(yaml-quote …)**
*   [Disable local loopback for volume host #181](https://github.com/kubernetes/cloud-provider-gcp/pull/181)
    *   **cluster/gce/gci/configure-helper.sh: --volume-host-allow-local-loopback**
*   [Bump cloud-provider-gcp to v1.21 #204](https://github.com/kubernetes/cloud-provider-gcp/pull/204)
    *   **cluster/gce/manifests/cloud-controller-manager.manifest: --log-file -> --log\_file**
*   [Use at minimum n1-standard-2 in cluster/gce/config-common.sh #228](https://github.com/kubernetes/cloud-provider-gcp/pull/228)
