load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "otlpconfig",
    srcs = [
        "envconfig.go",
        "options.go",
        "optiontypes.go",
        "tls.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc/internal/otlpconfig",
    importpath = "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc/internal/otlpconfig",
    visibility = ["//vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc:__subpackages__"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc/internal/envconfig",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc/internal/retry",
        "//vendor/go.opentelemetry.io/otel/internal/global",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/backoff",
        "//vendor/google.golang.org/grpc/credentials",
        "//vendor/google.golang.org/grpc/credentials/insecure",
        "//vendor/google.golang.org/grpc/encoding/gzip",
    ],
)
