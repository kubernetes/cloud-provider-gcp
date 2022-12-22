load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "unit",
    srcs = [
        "doc.go",
        "unit.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/metric/unit",
    importpath = "go.opentelemetry.io/otel/metric/unit",
    visibility = ["//visibility:public"],
)
