load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metric",
    srcs = [
        "atomicfields.go",
        "doc.go",
        "refcount_mapped.go",
        "sdk.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/metric",
    importpath = "go.opentelemetry.io/otel/sdk/metric",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel",
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/internal/metric",
        "//vendor/go.opentelemetry.io/otel/metric",
        "//vendor/go.opentelemetry.io/otel/metric/number",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/aggregator",
        "//vendor/go.opentelemetry.io/otel/sdk/resource",
    ],
)
