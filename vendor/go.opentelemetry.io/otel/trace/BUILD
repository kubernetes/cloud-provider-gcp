load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "trace",
    srcs = [
        "config.go",
        "context.go",
        "doc.go",
        "nonrecording.go",
        "noop.go",
        "trace.go",
        "tracestate.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/trace",
    importpath = "go.opentelemetry.io/otel/trace",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/codes",
    ],
)
