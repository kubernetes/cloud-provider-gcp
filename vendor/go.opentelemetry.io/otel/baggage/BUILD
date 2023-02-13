load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "baggage",
    srcs = [
        "baggage.go",
        "context.go",
        "doc.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/baggage",
    importpath = "go.opentelemetry.io/otel/baggage",
    visibility = ["//visibility:public"],
    deps = ["//vendor/go.opentelemetry.io/otel/internal/baggage"],
)
