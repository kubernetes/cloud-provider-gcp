load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "instrument",
    srcs = [
        "config.go",
        "instrument.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/metric/instrument",
    importpath = "go.opentelemetry.io/otel/metric/instrument",
    visibility = ["//visibility:public"],
    deps = ["//vendor/go.opentelemetry.io/otel/metric/unit"],
)
