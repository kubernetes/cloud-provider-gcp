load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "global",
    srcs = ["metric.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/metric/global",
    importpath = "go.opentelemetry.io/otel/metric/global",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/internal/global",
        "//vendor/go.opentelemetry.io/otel/metric",
    ],
)
