load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "envconfig",
    srcs = ["envconfig.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc/internal/envconfig",
    importpath = "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc/internal/envconfig",
    visibility = ["//vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc:__subpackages__"],
    deps = ["//vendor/go.opentelemetry.io/otel/internal/global"],
)
