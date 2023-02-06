load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "retry",
    srcs = ["retry.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp/internal/retry",
    importpath = "go.opentelemetry.io/otel/exporters/otlp/internal/retry",
    visibility = ["//vendor/go.opentelemetry.io/otel/exporters/otlp:__subpackages__"],
    deps = ["//vendor/github.com/cenkalti/backoff/v4:backoff"],
)
