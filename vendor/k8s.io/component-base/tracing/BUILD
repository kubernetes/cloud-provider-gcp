load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "tracing",
    srcs = [
        "tracing.go",
        "utils.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/tracing",
    importpath = "k8s.io/component-base/tracing",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc",
        "//vendor/go.opentelemetry.io/otel/propagation",
        "//vendor/go.opentelemetry.io/otel/sdk/resource",
        "//vendor/go.opentelemetry.io/otel/sdk/trace",
        "//vendor/go.opentelemetry.io/otel/semconv/v1.17.0:v1_17_0",
        "//vendor/go.opentelemetry.io/otel/trace",
        "//vendor/go.opentelemetry.io/otel/trace/noop",
        "//vendor/k8s.io/client-go/transport",
        "//vendor/k8s.io/component-base/tracing/api/v1:api",
        "//vendor/k8s.io/utils/trace",
    ],
)
