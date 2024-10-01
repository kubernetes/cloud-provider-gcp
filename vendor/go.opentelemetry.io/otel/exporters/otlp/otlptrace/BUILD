load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "otlptrace",
    srcs = [
        "clients.go",
        "doc.go",
        "exporter.go",
        "version.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace",
    importpath = "go.opentelemetry.io/otel/exporters/otlp/otlptrace",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/internal/tracetransform",
        "//vendor/go.opentelemetry.io/otel/sdk/trace",
        "//vendor/go.opentelemetry.io/proto/otlp/trace/v1:trace",
    ],
)
