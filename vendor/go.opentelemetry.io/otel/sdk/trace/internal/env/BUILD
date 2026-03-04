load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "env",
    srcs = ["env.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/trace/internal/env",
    importpath = "go.opentelemetry.io/otel/sdk/trace/internal/env",
    visibility = ["//vendor/go.opentelemetry.io/otel/sdk/trace:__subpackages__"],
    deps = ["//vendor/go.opentelemetry.io/otel/internal/global"],
)
