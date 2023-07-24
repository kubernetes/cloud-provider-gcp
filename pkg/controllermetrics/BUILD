load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "controllermetrics",
    srcs = ["controllermetrics.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/controllermetrics",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
    ],
)
