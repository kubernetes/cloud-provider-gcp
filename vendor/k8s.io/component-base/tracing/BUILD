load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["utils.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/tracing",
    importpath = "k8s.io/component-base/tracing",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp:go_default_library",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp:go_default_library",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/otlpgrpc:go_default_library",
        "//vendor/go.opentelemetry.io/otel/propagation:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/resource:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/trace:go_default_library",
        "//vendor/go.opentelemetry.io/otel/trace:go_default_library",
        "//vendor/k8s.io/client-go/transport:go_default_library",
        "//vendor/k8s.io/component-base/tracing/api/v1:go_default_library",
    ],
)
