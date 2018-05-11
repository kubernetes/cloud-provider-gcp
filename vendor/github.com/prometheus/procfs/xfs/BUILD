load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "parse.go",
        "xfs.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/prometheus/procfs/xfs",
    importpath = "github.com/prometheus/procfs/xfs",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/prometheus/procfs/internal/util:go_default_library"],
)
