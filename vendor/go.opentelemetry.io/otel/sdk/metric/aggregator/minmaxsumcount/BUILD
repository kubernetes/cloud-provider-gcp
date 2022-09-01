load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "minmaxsumcount",
    srcs = ["mmsc.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/metric/aggregator/minmaxsumcount",
    importpath = "go.opentelemetry.io/otel/sdk/metric/aggregator/minmaxsumcount",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/metric",
        "//vendor/go.opentelemetry.io/otel/metric/number",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric/aggregation",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/aggregator",
    ],
)
