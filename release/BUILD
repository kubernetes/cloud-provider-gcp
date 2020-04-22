package(default_visibility = ["//visibility:public"])

load("@io_k8s_repo_infra//defs:build.bzl", "release_filegroup")
load("@io_k8s_repo_infra//defs:pkg.bzl", "pkg_tar")

# We depend on the behavior that pkg_tar will use the first file that it sees
# in case of duplicates. I'm open for building something with more explicit
# support for overlaying release artifacts.
pkg_tar(
    name = "kubernetes-server-linux-amd64",
    srcs = [
        "//cmd/cloud-controller-manager",
        "//cmd/cloud-controller-manager:cloud-controller-manager.docker_tag",
        ":rename_ccm_image",
    ],
    extension = "tar.gz",
    remap_paths = {
        "/cmd/cloud-controller-manager/cloud-controller-manager.tar": "kubernetes/server/bin/cloud-controller-manager.tar",
        "/cloud-controller-manager.docker_tag": "kubernetes/server/bin/cloud-controller-manager.docker_tag",
        "/cloud-controller-manager": "kubernetes/server/bin/cloud-controller-manager",
    },
    deps = [
        "@io_k8s_release//:kubernetes-server-linux-amd64",
        ":rename_ccm_image",
    ],
)

genrule(
    name = "rename_ccm_image",
    srcs = ["//cmd/cloud-controller-manager:image.tar"],
    outs = ["cmd/cloud-controller-manager/cloud-controller-manager.tar"],
    output_to_bindir = 1,
    cmd = "mv $< $@",
)

genrule(
    name = "gen_server_shasum",
    srcs = [
        ":kubernetes-server-linux-amd64.tar.gz",
    ],
    outs = [
        ":kubernetes-server-linux-amd64.tar.gz.sha512",
    ],
    output_to_bindir = 1,
    cmd = "shasum -a512 $< > $<.sha512",
)

genrule(
    name = "gen_manifest_shasum",
    srcs = [
        ":kubernetes-manifests.tar.gz",
    ],
    outs = [
        ":kubernetes-manifests.tar.gz.sha512",
    ],
    output_to_bindir = 1,
    cmd = "shasum -a512 $< > $<.sha512",
)

pkg_tar(
    name = "kubernetes-manifests",
    extension = "tar.gz",
    deps = [
        "//deploy:addon",
        "//deploy:manifest",
        "//cluster:manifests",
        "@io_k8s_release//:kubernetes-manifests",
    ],
)

release_filegroup(
    name = "release-tars",
    srcs = [
        ":kubernetes-manifests.tar.gz.sha512",
        ":kubernetes-server-linux-amd64.tar.gz.sha512",
        "@io_k8s_release//:kubernetes-node-linux-amd64",
    ],
)
