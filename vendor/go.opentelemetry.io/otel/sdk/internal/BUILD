load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = [
        "gen.go",
        "internal.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/internal",
    importpath = "go.opentelemetry.io/otel/sdk/internal",
    visibility = ["//vendor/go.opentelemetry.io/otel/sdk:__subpackages__"],
)
