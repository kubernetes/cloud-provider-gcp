load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "otelconv",
    srcs = ["metric.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/semconv/v1.39.0/otelconv",
    importpath = "go.opentelemetry.io/otel/semconv/v1.39.0/otelconv",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/metric",
        "//vendor/go.opentelemetry.io/otel/metric/noop",
    ],
)
