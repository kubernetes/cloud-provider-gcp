load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "otlp",
    srcs = [
        "doc.go",
        "options.go",
        "optiontypes.go",
        "otlp.go",
        "protocoldriver.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp",
    importpath = "go.opentelemetry.io/otel/exporters/otlp",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel",
        "//vendor/go.opentelemetry.io/otel/metric",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric/aggregation",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/controller/basic",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/processor/basic",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/selector/simple",
        "//vendor/go.opentelemetry.io/otel/sdk/trace",
    ],
)
