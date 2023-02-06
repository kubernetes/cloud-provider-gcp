load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "syncint64",
    srcs = ["syncint64.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/metric/instrument/syncint64",
    importpath = "go.opentelemetry.io/otel/metric/instrument/syncint64",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/metric/instrument",
    ],
)
