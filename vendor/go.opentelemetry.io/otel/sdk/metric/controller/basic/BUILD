load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "basic",
    srcs = [
        "config.go",
        "controller.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/metric/controller/basic",
    importpath = "go.opentelemetry.io/otel/sdk/metric/controller/basic",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel",
        "//vendor/go.opentelemetry.io/otel/metric",
        "//vendor/go.opentelemetry.io/otel/metric/registry",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric",
        "//vendor/go.opentelemetry.io/otel/sdk/metric",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/controller/time",
        "//vendor/go.opentelemetry.io/otel/sdk/resource",
    ],
)
