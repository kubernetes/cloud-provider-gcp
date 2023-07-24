load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "workqueue",
    srcs = ["metrics.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/metrics/prometheus/workqueue",
    importpath = "k8s.io/component-base/metrics/prometheus/workqueue",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/client-go/util/workqueue",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
    ],
)
