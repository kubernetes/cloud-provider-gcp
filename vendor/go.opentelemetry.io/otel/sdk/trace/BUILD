load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "trace",
    srcs = [
        "attributesmap.go",
        "batch_span_processor.go",
        "config.go",
        "doc.go",
        "evictedqueue.go",
        "id_generator.go",
        "provider.go",
        "sampling.go",
        "simple_span_processor.go",
        "span.go",
        "span_exporter.go",
        "span_processor.go",
        "tracer.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/trace",
    importpath = "go.opentelemetry.io/otel/sdk/trace",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel",
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/codes",
        "//vendor/go.opentelemetry.io/otel/sdk/instrumentation",
        "//vendor/go.opentelemetry.io/otel/sdk/internal",
        "//vendor/go.opentelemetry.io/otel/sdk/resource",
        "//vendor/go.opentelemetry.io/otel/semconv",
        "//vendor/go.opentelemetry.io/otel/trace",
    ],
)
