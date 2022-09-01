load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "leaderelection",
    srcs = ["metrics.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/metrics/prometheus/clientgo/leaderelection",
    importpath = "k8s.io/component-base/metrics/prometheus/clientgo/leaderelection",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/client-go/tools/leaderelection",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
    ],
)
