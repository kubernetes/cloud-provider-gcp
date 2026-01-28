load("@rules_pkg//pkg:tar.bzl", "pkg_tar")

package(default_visibility = ["//visibility:public"])

pkg_tar(
    name = "manifest",
    srcs = ["cloud-controller-manager.manifest"],
    package_dir = "./kubernetes/gci-trusty",
)

pkg_tar(
    name = "addon",
    srcs = [
        "cloud-node-controller-binding.yaml",
        "cloud-node-controller-role.yaml",
        "pvl-controller-role.yaml",
    ],
    package_dir = "./kubernetes/gci-trusty/cloud-controller-manager",
)
