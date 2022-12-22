load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "slis",
    srcs = [
        "metrics.go",
        "registry.go",
        "routes.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/metrics/prometheus/slis",
    importpath = "k8s.io/component-base/metrics/prometheus/slis",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/component-base/metrics"],
)
