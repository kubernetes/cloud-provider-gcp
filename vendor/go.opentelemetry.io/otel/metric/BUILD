load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "config.go",
        "doc.go",
        "instrumentkind_string.go",
        "metric.go",
        "metric_instrument.go",
        "metric_noop.go",
        "metric_sdkapi.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/metric",
    importpath = "go.opentelemetry.io/otel/metric",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute:go_default_library",
        "//vendor/go.opentelemetry.io/otel/metric/number:go_default_library",
        "//vendor/go.opentelemetry.io/otel/unit:go_default_library",
    ],
)
