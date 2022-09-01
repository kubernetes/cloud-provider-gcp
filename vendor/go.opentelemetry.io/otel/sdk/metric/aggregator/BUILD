load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "aggregator",
    srcs = ["aggregator.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/metric/aggregator",
    importpath = "go.opentelemetry.io/otel/sdk/metric/aggregator",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/metric",
        "//vendor/go.opentelemetry.io/otel/metric/number",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric/aggregation",
    ],
)
