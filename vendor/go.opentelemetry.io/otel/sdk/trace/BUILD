load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
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
        "//vendor/go.opentelemetry.io/otel:go_default_library",
        "//vendor/go.opentelemetry.io/otel/attribute:go_default_library",
        "//vendor/go.opentelemetry.io/otel/codes:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/instrumentation:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/internal:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/resource:go_default_library",
        "//vendor/go.opentelemetry.io/otel/semconv:go_default_library",
        "//vendor/go.opentelemetry.io/otel/trace:go_default_library",
    ],
)
