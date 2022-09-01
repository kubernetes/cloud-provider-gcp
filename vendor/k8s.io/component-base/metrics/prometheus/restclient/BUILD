load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "restclient",
    srcs = ["metrics.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/metrics/prometheus/restclient",
    importpath = "k8s.io/component-base/metrics/prometheus/restclient",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/client-go/tools/metrics",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
    ],
)
