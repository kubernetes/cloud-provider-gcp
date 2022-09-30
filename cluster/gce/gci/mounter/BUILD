package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)

go_binary(
    name = "mounter",
    embed = [":mounter_lib"],
)

go_library(
    name = "mounter_lib",
    srcs = ["mounter.go"],
    importpath = "k8s.io/cloud-provider-gcp/cluster/gce/gci/mounter",
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
