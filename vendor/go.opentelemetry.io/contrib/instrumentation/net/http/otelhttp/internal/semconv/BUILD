load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "semconv",
    srcs = [
        "env.go",
        "httpconv.go",
        "util.go",
        "v1.20.0.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp/internal/semconv",
    importpath = "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp/internal/semconv",
    visibility = ["//vendor/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp:__subpackages__"],
    deps = [
        "//vendor/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp/internal/semconvutil",
        "//vendor/go.opentelemetry.io/otel",
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/codes",
        "//vendor/go.opentelemetry.io/otel/metric",
        "//vendor/go.opentelemetry.io/otel/metric/noop",
        "//vendor/go.opentelemetry.io/otel/semconv/v1.20.0:v1_20_0",
        "//vendor/go.opentelemetry.io/otel/semconv/v1.26.0:v1_26_0",
    ],
)
