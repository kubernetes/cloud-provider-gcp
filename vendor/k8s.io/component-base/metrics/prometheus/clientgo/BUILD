load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "clientgo",
    srcs = ["metrics.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/metrics/prometheus/clientgo",
    importpath = "k8s.io/component-base/metrics/prometheus/clientgo",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/component-base/metrics/prometheus/clientgo/leaderelection",
        "//vendor/k8s.io/component-base/metrics/prometheus/restclient",
        "//vendor/k8s.io/component-base/metrics/prometheus/workqueue",
    ],
)
