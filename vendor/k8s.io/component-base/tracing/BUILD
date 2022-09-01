load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "tracing",
    srcs = ["utils.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/tracing",
    importpath = "k8s.io/component-base/tracing",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/otlpgrpc",
        "//vendor/go.opentelemetry.io/otel/propagation",
        "//vendor/go.opentelemetry.io/otel/sdk/resource",
        "//vendor/go.opentelemetry.io/otel/sdk/trace",
        "//vendor/go.opentelemetry.io/otel/trace",
        "//vendor/k8s.io/client-go/transport",
        "//vendor/k8s.io/component-base/tracing/api/v1:api",
    ],
)
