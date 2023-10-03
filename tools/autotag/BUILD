load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "autotag_lib",
    srcs = ["main.go"],
    importpath = "k8s.io/cloud-provider-gcp/tools/autotag",
    visibility = ["//visibility:private"],
    deps = ["//vendor/k8s.io/klog/v2:klog"],
)

go_binary(
    name = "autotag",
    embed = [":autotag_lib"],
    visibility = ["//visibility:public"],
)
