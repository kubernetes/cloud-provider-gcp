load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = [
        "gen.go",
        "partialsuccess.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc/internal",
    importpath = "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc/internal",
    visibility = ["//vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc:__subpackages__"],
)
