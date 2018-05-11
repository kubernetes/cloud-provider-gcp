load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "executor.go",
        "go_above_19.go",
        "go_below_19.go",
        "log.go",
        "unbounded_executor.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/modern-go/concurrent",
    importpath = "github.com/modern-go/concurrent",
    visibility = ["//visibility:public"],
)
