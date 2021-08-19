load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "attribute.go",
        "instrumentation.go",
        "metric.go",
        "resource.go",
        "span.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp/internal/transform",
    importpath = "go.opentelemetry.io/otel/exporters/otlp/internal/transform",
    visibility = ["//vendor/go.opentelemetry.io/otel/exporters/otlp:__subpackages__"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute:go_default_library",
        "//vendor/go.opentelemetry.io/otel/codes:go_default_library",
        "//vendor/go.opentelemetry.io/otel/metric/number:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric/aggregation:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/instrumentation:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/resource:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/trace:go_default_library",
        "//vendor/go.opentelemetry.io/otel/trace:go_default_library",
        "//vendor/go.opentelemetry.io/proto/otlp/common/v1:go_default_library",
        "//vendor/go.opentelemetry.io/proto/otlp/metrics/v1:go_default_library",
        "//vendor/go.opentelemetry.io/proto/otlp/resource/v1:go_default_library",
        "//vendor/go.opentelemetry.io/proto/otlp/trace/v1:go_default_library",
    ],
)
