load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "sdk",
    srcs = [
        "doc.go",
        "limit.go",
        "span.go",
        "tracer.go",
        "tracer_provider.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/auto/sdk",
    importpath = "go.opentelemetry.io/auto/sdk",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/auto/sdk/internal/telemetry",
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/codes",
        "//vendor/go.opentelemetry.io/otel/semconv/v1.26.0:v1_26_0",
        "//vendor/go.opentelemetry.io/otel/trace",
        "//vendor/go.opentelemetry.io/otel/trace/noop",
    ],
)
