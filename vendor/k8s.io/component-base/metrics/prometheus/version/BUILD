load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["metrics.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/metrics/prometheus/version",
    importpath = "k8s.io/component-base/metrics/prometheus/version",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/component-base/metrics:go_default_library",
        "//vendor/k8s.io/component-base/metrics/legacyregistry:go_default_library",
        "//vendor/k8s.io/component-base/version:go_default_library",
    ],
)
