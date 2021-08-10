load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["aggregation.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/export/metric/aggregation",
    importpath = "go.opentelemetry.io/otel/sdk/export/metric/aggregation",
    visibility = ["//visibility:public"],
    deps = ["//vendor/go.opentelemetry.io/otel/metric/number:go_default_library"],
)
