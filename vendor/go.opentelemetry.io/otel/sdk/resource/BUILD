load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "resource",
    srcs = [
        "auto.go",
        "builtin.go",
        "config.go",
        "doc.go",
        "env.go",
        "os.go",
        "process.go",
        "resource.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/sdk/resource",
    importpath = "go.opentelemetry.io/otel/sdk/resource",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel",
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/semconv",
    ],
)
