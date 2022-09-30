load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "noop",
    srcs = ["noop.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/internal/trace/noop",
    importpath = "go.opentelemetry.io/otel/internal/trace/noop",
    visibility = ["//vendor/go.opentelemetry.io/otel:__subpackages__"],
    deps = ["//vendor/go.opentelemetry.io/otel/trace"],
)
