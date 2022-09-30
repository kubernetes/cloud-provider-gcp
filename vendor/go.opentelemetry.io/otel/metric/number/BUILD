load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "number",
    srcs = [
        "doc.go",
        "kind_string.go",
        "number.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/metric/number",
    importpath = "go.opentelemetry.io/otel/metric/number",
    visibility = ["//visibility:public"],
    deps = ["//vendor/go.opentelemetry.io/otel/internal"],
)
