load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "buffer",
    srcs = [
        "buffer.go",
        "pool.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.uber.org/zap/buffer",
    importpath = "go.uber.org/zap/buffer",
    visibility = ["//visibility:public"],
    deps = ["//vendor/go.uber.org/zap/internal/pool"],
)
