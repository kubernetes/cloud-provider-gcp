load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "compatversion",
    srcs = ["metrics.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/metrics/prometheus/compatversion",
    importpath = "k8s.io/component-base/metrics/prometheus/compatversion",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
    ],
)
