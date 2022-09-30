load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "legacyregistry",
    srcs = ["registry.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/metrics/legacyregistry",
    importpath = "k8s.io/component-base/metrics/legacyregistry",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/prometheus/client_golang/prometheus",
        "//vendor/github.com/prometheus/client_golang/prometheus/promhttp",
        "//vendor/k8s.io/component-base/metrics",
    ],
)
