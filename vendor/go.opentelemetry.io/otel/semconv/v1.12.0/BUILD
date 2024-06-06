load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v1_12_0",
    srcs = [
        "doc.go",
        "exception.go",
        "http.go",
        "resource.go",
        "schema.go",
        "trace.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/semconv/v1.12.0",
    importpath = "go.opentelemetry.io/otel/semconv/v1.12.0",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/codes",
        "//vendor/go.opentelemetry.io/otel/semconv/internal",
        "//vendor/go.opentelemetry.io/otel/trace",
    ],
)
