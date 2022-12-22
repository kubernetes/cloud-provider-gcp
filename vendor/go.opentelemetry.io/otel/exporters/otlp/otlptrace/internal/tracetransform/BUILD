load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "tracetransform",
    srcs = [
        "attribute.go",
        "instrumentation.go",
        "resource.go",
        "span.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/internal/tracetransform",
    importpath = "go.opentelemetry.io/otel/exporters/otlp/otlptrace/internal/tracetransform",
    visibility = ["//vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace:__subpackages__"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/codes",
        "//vendor/go.opentelemetry.io/otel/sdk/instrumentation",
        "//vendor/go.opentelemetry.io/otel/sdk/resource",
        "//vendor/go.opentelemetry.io/otel/sdk/trace",
        "//vendor/go.opentelemetry.io/otel/trace",
        "//vendor/go.opentelemetry.io/proto/otlp/common/v1:common",
        "//vendor/go.opentelemetry.io/proto/otlp/resource/v1:resource",
        "//vendor/go.opentelemetry.io/proto/otlp/trace/v1:trace",
    ],
)
