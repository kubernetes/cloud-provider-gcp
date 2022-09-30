load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "basic",
    srcs = [
        "basic.go",
        "config.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/metric/processor/basic",
    importpath = "go.opentelemetry.io/otel/sdk/metric/processor/basic",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/metric",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric/aggregation",
        "//vendor/go.opentelemetry.io/otel/sdk/resource",
    ],
)
