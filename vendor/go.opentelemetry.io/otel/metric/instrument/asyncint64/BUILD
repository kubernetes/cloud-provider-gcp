load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "asyncint64",
    srcs = ["asyncint64.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/metric/instrument/asyncint64",
    importpath = "go.opentelemetry.io/otel/metric/instrument/asyncint64",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/metric/instrument",
    ],
)
