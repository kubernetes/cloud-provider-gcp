load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["library.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/instrumentation",
    importpath = "go.opentelemetry.io/otel/sdk/instrumentation",
    visibility = ["//visibility:public"],
)
