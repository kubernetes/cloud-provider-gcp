load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "client.go",
        "common.go",
        "config.go",
        "doc.go",
        "handler.go",
        "labeler.go",
        "transport.go",
        "wrap.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
    importpath = "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/felixge/httpsnoop:go_default_library",
        "//vendor/go.opentelemetry.io/contrib:go_default_library",
        "//vendor/go.opentelemetry.io/otel:go_default_library",
        "//vendor/go.opentelemetry.io/otel/attribute:go_default_library",
        "//vendor/go.opentelemetry.io/otel/metric:go_default_library",
        "//vendor/go.opentelemetry.io/otel/metric/global:go_default_library",
        "//vendor/go.opentelemetry.io/otel/propagation:go_default_library",
        "//vendor/go.opentelemetry.io/otel/semconv:go_default_library",
        "//vendor/go.opentelemetry.io/otel/trace:go_default_library",
    ],
)
