load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "noop",
    srcs = ["noop.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/metric/noop",
    importpath = "go.opentelemetry.io/otel/metric/noop",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/metric",
        "//vendor/go.opentelemetry.io/otel/metric/embedded",
    ],
)
