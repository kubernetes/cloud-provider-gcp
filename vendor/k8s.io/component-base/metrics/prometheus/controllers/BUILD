load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["metrics.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/metrics/prometheus/controllers",
    importpath = "k8s.io/component-base/metrics/prometheus/controllers",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/component-base/metrics:go_default_library",
        "//vendor/k8s.io/component-base/metrics/legacyregistry:go_default_library",
    ],
)
