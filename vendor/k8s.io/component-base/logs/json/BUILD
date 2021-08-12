load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["json.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/logs/json",
    importpath = "k8s.io/component-base/logs/json",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/go-logr/logr:go_default_library",
        "//vendor/go.uber.org/zap:go_default_library",
        "//vendor/go.uber.org/zap/zapcore:go_default_library",
    ],
)
