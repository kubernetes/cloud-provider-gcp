package(default_visibility = ["//visibility:public"])

load("@rules_pkg//pkg:tar.bzl", "pkg_tar")

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [
        ":package-srcs",
        "//cluster/addons:all-srcs",
        "//cluster/gce:all-srcs",
    ],
    tags = ["automanaged"],
)

pkg_tar(
    name = "manifests",
    mode = "0644",
    package_dir = "kubernetes/gci-trusty",
    deps = [
        "//cluster/addons",
        "//cluster/gce/addons",
        "//cluster/gce/gci:gci-trusty-manifests",
        "//cluster/gce/manifests:gce-master-manifests",
    ],
)

sh_library(
    name = "lib",
    srcs = [
        "util.sh",
    ],
)

# These tests just verify that bash can interpret the file.
sh_test(
    name = "common_test",
    srcs = ["common.sh"],
    deps = [
        "//cluster:lib",
    ],
)

sh_test(
    name = "clientbin_test",
    srcs = ["clientbin.sh"],
)

sh_test(
    name = "kube-util_test",
    srcs = ["kube-util.sh"],
)
