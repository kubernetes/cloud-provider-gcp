load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metric",
    srcs = ["async.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/internal/metric",
    importpath = "go.opentelemetry.io/otel/internal/metric",
    visibility = ["//vendor/go.opentelemetry.io/otel:__subpackages__"],
    deps = [
        "//vendor/go.opentelemetry.io/otel",
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/metric",
    ],
)
