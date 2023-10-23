load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = ["rawhelpers.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/internal",
    importpath = "go.opentelemetry.io/otel/internal",
    visibility = ["//vendor/go.opentelemetry.io/otel:__subpackages__"],
)
