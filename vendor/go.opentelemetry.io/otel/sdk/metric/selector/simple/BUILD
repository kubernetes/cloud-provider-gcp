load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["simple.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/metric/selector/simple",
    importpath = "go.opentelemetry.io/otel/sdk/metric/selector/simple",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/metric:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/aggregator/exact:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/aggregator/histogram:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/aggregator/lastvalue:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/aggregator/minmaxsumcount:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/aggregator/sum:go_default_library",
    ],
)
