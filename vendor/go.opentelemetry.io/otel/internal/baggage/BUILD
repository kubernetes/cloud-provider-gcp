load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "baggage",
    srcs = ["baggage.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/internal/baggage",
    importpath = "go.opentelemetry.io/otel/internal/baggage",
    visibility = ["//vendor/go.opentelemetry.io/otel:__subpackages__"],
    deps = ["//vendor/go.opentelemetry.io/otel/attribute"],
)
