package(default_visibility = ["//visibility:public"])

load("@io_k8s_repo_infra//defs:build.bzl", "release_filegroup")
load("@rules_pkg//pkg:tar.bzl", "pkg_tar")

# We depend on the behavior that pkg_tar will use the first file that it sees
# in case of duplicates. I'm open for building something with more explicit
# support for overlaying release artifacts.
pkg_tar(
    name = "kubernetes-server-linux-amd64",
    srcs = [
        ":rename_ccm_image",
        "//cmd/auth-provider-gcp",
        "//cmd/cloud-controller-manager",
        "//cmd/cloud-controller-manager:cloud-controller-manager.docker_tag",
    ],
    extension = "tar.gz",
    remap_paths = {
        "/auth-provider-gcp": "kubernetes/server/bin/auth-provider-gcp",
        "/cmd/cloud-controller-manager/cloud-controller-manager.tar": "kubernetes/server/bin/cloud-controller-manager.tar",
        "/cloud-controller-manager.docker_tag": "kubernetes/server/bin/cloud-controller-manager.docker_tag",
        "/cloud-controller-manager": "kubernetes/server/bin/cloud-controller-manager",
    },
    deps = [
        ":rename_ccm_image",
        "@io_k8s_release//:kubernetes-server-linux-amd64",
    ],
)

pkg_tar(
    name = "kubernetes-node-windows-amd64",
    srcs = [
        "//cmd/auth-provider-gcp",
    ],
    extension = "tar.gz",
    remap_paths = {
        "/auth-provider-gcp.exe": "kubernetes/node/bin/auth-provider-gcp.exe",
    },
    deps = [
        "@io_k8s_release//:kubernetes-node-windows-amd64",
    ],
)

genrule(
    name = "rename_ccm_image",
    srcs = ["//cmd/cloud-controller-manager:image.tar"],
    outs = ["cmd/cloud-controller-manager/cloud-controller-manager.tar"],
    cmd = '''
    {
      mkdir archive
      cd archive
      tar tf ../$< > contents
      tar xf ../$<
      image_tag=`awk '/STABLE_IMAGE_TAG/ {print $$2}' ../bazel-out/stable-status.txt`
      # Fix the image tag to not contain '+'
      image_tag=`echo $$image_tag | sed 's/+/-/g'`
      sed -i "s@:image\"@:$${image_tag}\"@g" manifest.json
      sed -i "s@\"image\"@\"$${image_tag}\"@g" repositories
      sed -i "s@registry.k8s.io/cmd/cloud-controller-manager:@registry.k8s.io/cloud-controller-manager:@g" manifest.json
      sed -i "s@registry.k8s.io/cmd/cloud-controller-manager@registry.k8s.io/cloud-controller-manager@g" repositories
      tar -c -T contents -f ../$@
      cd ..
    }''',
    output_to_bindir = 1,
    stamp = 1,
)

genrule(
    name = "gen_server_shasum",
    srcs = [
        ":kubernetes-server-linux-amd64.tar.gz",
    ],
    outs = [
        ":kubernetes-server-linux-amd64.tar.gz.sha512",
    ],
    cmd = "shasum -a512 $< > $<.sha512",
    output_to_bindir = 1,
)

genrule(
    name = "gen_manifest_shasum",
    srcs = [
        ":kubernetes-manifests.tar.gz",
    ],
    outs = [
        ":kubernetes-manifests.tar.gz.sha512",
    ],
    cmd = "shasum -a512 $< > $<.sha512",
    output_to_bindir = 1,
)

genrule(
    name = "gen_windows_node_shasum",
    srcs = [
        ":kubernetes-node-windows-amd64.tar.gz",
    ],
    outs = [
        ":kubernetes-node-windows-amd64.tar.gz.sha512",
    ],
    cmd = "shasum -a512 $< > $<.sha512",
    output_to_bindir = 1,
)

pkg_tar(
    name = "kubernetes-manifests",
    extension = "tar.gz",
    deps = [
        "//cluster:manifests",
        "//deploy:addon",
        "//deploy:manifest",
        "@io_k8s_release//:kubernetes-manifests",
    ],
)

release_filegroup(
    name = "release-tars",
    conditioned_srcs = {
        "@io_bazel_rules_go//go/platform:linux_amd64": [
            ":kubernetes-manifests.tar.gz.sha512",
            ":kubernetes-server-linux-amd64.tar.gz.sha512",
            "@io_k8s_release//:kubernetes-node-linux-amd64",
        ],
        "@io_bazel_rules_go//go/platform:windows_amd64": [
            ":kubernetes-node-windows-amd64.tar.gz.sha512",
            "@io_k8s_release//:kubernetes-node-windows-amd64",
        ],
    },
)
