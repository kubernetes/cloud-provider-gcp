load("@bazel_tools//tools/build_defs/pkg:pkg.bzl", "pkg_tar")

package(default_visibility = ["//visibility:public"])

pkg_tar(
    name = "manifest",
    srcs = ["cloud-controller-manager.manifest"],
    package_dir = "./kubernetes/gci-trusty",
)

pkg_tar(
    name = "addon",
    srcs = [
        "manifest.yaml",
    ],
    package_dir = "./kubernetes/gci-trusty/cloud-controller-manager",
)
