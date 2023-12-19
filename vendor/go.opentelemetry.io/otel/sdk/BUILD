load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "sdk",
    srcs = ["version.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk",
    importpath = "go.opentelemetry.io/otel/sdk",
    visibility = ["//visibility:public"],
)
