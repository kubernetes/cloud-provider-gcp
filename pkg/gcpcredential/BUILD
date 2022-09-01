package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "gcpcredential",
    srcs = ["gcpcredential.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/gcpcredential",
    deps = [
        "//pkg/credentialconfig",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
)
