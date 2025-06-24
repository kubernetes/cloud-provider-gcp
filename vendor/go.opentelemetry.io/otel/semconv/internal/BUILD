load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = ["http.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/semconv/internal",
    importpath = "go.opentelemetry.io/otel/semconv/internal",
    visibility = ["//vendor/go.opentelemetry.io/otel/semconv:__subpackages__"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/codes",
        "//vendor/go.opentelemetry.io/otel/trace",
    ],
)
