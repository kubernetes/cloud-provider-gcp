load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "stacktrace",
    srcs = ["stack.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.uber.org/zap/internal/stacktrace",
    importpath = "go.uber.org/zap/internal/stacktrace",
    visibility = ["//vendor/go.uber.org/zap:__subpackages__"],
    deps = [
        "//vendor/go.uber.org/zap/buffer",
        "//vendor/go.uber.org/zap/internal/bufferpool",
        "//vendor/go.uber.org/zap/internal/pool",
    ],
)
