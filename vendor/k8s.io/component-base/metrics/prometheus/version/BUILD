load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "version",
    srcs = ["metrics.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/metrics/prometheus/version",
    importpath = "k8s.io/component-base/metrics/prometheus/version",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
        "//vendor/k8s.io/component-base/version",
    ],
)
