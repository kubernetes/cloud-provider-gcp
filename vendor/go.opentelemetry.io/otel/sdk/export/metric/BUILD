load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metric",
    srcs = [
        "exportkind_string.go",
        "metric.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/export/metric",
    importpath = "go.opentelemetry.io/otel/sdk/export/metric",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/metric",
        "//vendor/go.opentelemetry.io/otel/metric/number",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric/aggregation",
        "//vendor/go.opentelemetry.io/otel/sdk/resource",
    ],
)
