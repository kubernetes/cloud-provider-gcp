load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "simple",
    srcs = ["simple.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/metric/selector/simple",
    importpath = "go.opentelemetry.io/otel/sdk/metric/selector/simple",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/metric",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/aggregator/exact",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/aggregator/histogram",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/aggregator/lastvalue",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/aggregator/minmaxsumcount",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/aggregator/sum",
    ],
)
