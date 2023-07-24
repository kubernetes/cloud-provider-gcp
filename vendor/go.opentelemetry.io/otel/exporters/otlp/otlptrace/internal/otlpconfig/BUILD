load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "otlpconfig",
    srcs = [
        "envconfig.go",
        "options.go",
        "optiontypes.go",
        "tls.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/internal/otlpconfig",
    importpath = "go.opentelemetry.io/otel/exporters/otlp/otlptrace/internal/otlpconfig",
    visibility = ["//vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace:__subpackages__"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/internal",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/internal/envconfig",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/internal/retry",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/backoff",
        "//vendor/google.golang.org/grpc/credentials",
        "//vendor/google.golang.org/grpc/credentials/insecure",
        "//vendor/google.golang.org/grpc/encoding/gzip",
    ],
)
