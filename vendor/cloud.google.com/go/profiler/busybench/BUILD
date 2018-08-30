load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["busybench.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/profiler/busybench",
    importpath = "cloud.google.com/go/profiler/busybench",
    visibility = ["//visibility:private"],
    deps = ["//vendor/cloud.google.com/go/profiler:go_default_library"],
)

go_binary(
    name = "busybench",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
