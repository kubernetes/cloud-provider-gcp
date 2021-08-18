load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "internal.go",
        "sanitize.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/internal",
    importpath = "go.opentelemetry.io/otel/sdk/internal",
    visibility = ["//vendor/go.opentelemetry.io/otel/sdk:__subpackages__"],
    deps = ["//vendor/go.opentelemetry.io/otel:go_default_library"],
)
