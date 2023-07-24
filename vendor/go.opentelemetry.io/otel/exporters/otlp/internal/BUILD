load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = [
        "config.go",
        "partialsuccess.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp/internal",
    importpath = "go.opentelemetry.io/otel/exporters/otlp/internal",
    visibility = ["//vendor/go.opentelemetry.io/otel/exporters/otlp:__subpackages__"],
)
