load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["exact.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/metric/aggregator/exact",
    importpath = "go.opentelemetry.io/otel/sdk/metric/aggregator/exact",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/metric:go_default_library",
        "//vendor/go.opentelemetry.io/otel/metric/number:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric/aggregation:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/aggregator:go_default_library",
    ],
)
