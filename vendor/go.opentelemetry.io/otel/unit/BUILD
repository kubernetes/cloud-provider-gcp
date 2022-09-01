load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "unit",
    srcs = [
        "doc.go",
        "unit.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/unit",
    importpath = "go.opentelemetry.io/otel/unit",
    visibility = ["//visibility:public"],
)
