load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "otel",
    srcs = [
        "doc.go",
        "error_handler.go",
        "handler.go",
        "internal_logging.go",
        "propagation.go",
        "trace.go",
        "version.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel",
    importpath = "go.opentelemetry.io/otel",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/go-logr/logr",
        "//vendor/go.opentelemetry.io/otel/internal/global",
        "//vendor/go.opentelemetry.io/otel/propagation",
        "//vendor/go.opentelemetry.io/otel/trace",
    ],
)
