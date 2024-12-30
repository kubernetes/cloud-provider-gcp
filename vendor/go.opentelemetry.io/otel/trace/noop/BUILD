load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "noop",
    srcs = ["noop.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/trace/noop",
    importpath = "go.opentelemetry.io/otel/trace/noop",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/codes",
        "//vendor/go.opentelemetry.io/otel/trace",
        "//vendor/go.opentelemetry.io/otel/trace/embedded",
    ],
)
