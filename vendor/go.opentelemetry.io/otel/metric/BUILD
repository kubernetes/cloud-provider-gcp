load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metric",
    srcs = [
        "config.go",
        "doc.go",
        "meter.go",
        "noop.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/metric",
    importpath = "go.opentelemetry.io/otel/metric",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/metric/instrument",
        "//vendor/go.opentelemetry.io/otel/metric/instrument/asyncfloat64",
        "//vendor/go.opentelemetry.io/otel/metric/instrument/asyncint64",
        "//vendor/go.opentelemetry.io/otel/metric/instrument/syncfloat64",
        "//vendor/go.opentelemetry.io/otel/metric/instrument/syncint64",
    ],
)
