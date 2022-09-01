load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "registry",
    srcs = [
        "doc.go",
        "registry.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/metric/registry",
    importpath = "go.opentelemetry.io/otel/metric/registry",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/metric",
    ],
)
