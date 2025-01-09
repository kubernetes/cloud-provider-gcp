load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "otelhttp",
    srcs = [
        "client.go",
        "common.go",
        "config.go",
        "doc.go",
        "handler.go",
        "labeler.go",
        "start_time_context.go",
        "transport.go",
        "version.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
    importpath = "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/felixge/httpsnoop",
        "//vendor/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp/internal/request",
        "//vendor/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp/internal/semconv",
        "//vendor/go.opentelemetry.io/otel",
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/codes",
        "//vendor/go.opentelemetry.io/otel/metric",
        "//vendor/go.opentelemetry.io/otel/propagation",
        "//vendor/go.opentelemetry.io/otel/trace",
    ],
)
