load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "asyncfloat64",
    srcs = ["asyncfloat64.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/metric/instrument/asyncfloat64",
    importpath = "go.opentelemetry.io/otel/metric/instrument/asyncfloat64",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/metric/instrument",
    ],
)
