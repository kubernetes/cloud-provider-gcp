load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "envconfig.go",
        "options.go",
        "tls.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp/internal/otlpconfig",
    importpath = "go.opentelemetry.io/otel/exporters/otlp/internal/otlpconfig",
    visibility = ["//vendor/go.opentelemetry.io/otel/exporters/otlp:__subpackages__"],
    deps = [
        "//vendor/go.opentelemetry.io/otel:go_default_library",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp:go_default_library",
        "//vendor/google.golang.org/grpc:go_default_library",
        "//vendor/google.golang.org/grpc/credentials:go_default_library",
    ],
)
