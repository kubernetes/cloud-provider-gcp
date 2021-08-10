load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "baggage.go",
        "doc.go",
        "propagation.go",
        "trace_context.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/propagation",
    importpath = "go.opentelemetry.io/otel/propagation",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute:go_default_library",
        "//vendor/go.opentelemetry.io/otel/internal/baggage:go_default_library",
        "//vendor/go.opentelemetry.io/otel/trace:go_default_library",
    ],
)
