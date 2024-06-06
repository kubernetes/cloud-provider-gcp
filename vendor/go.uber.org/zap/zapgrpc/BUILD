load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "zapgrpc",
    srcs = ["zapgrpc.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.uber.org/zap/zapgrpc",
    importpath = "go.uber.org/zap/zapgrpc",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.uber.org/zap",
        "//vendor/go.uber.org/zap/zapcore",
    ],
)
