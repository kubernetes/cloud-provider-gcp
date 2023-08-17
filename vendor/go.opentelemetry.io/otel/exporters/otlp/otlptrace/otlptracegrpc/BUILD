load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "otlptracegrpc",
    srcs = [
        "client.go",
        "exporter.go",
        "options.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc",
    importpath = "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/internal",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/internal/retry",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/otlptrace/internal/otlpconfig",
        "//vendor/go.opentelemetry.io/proto/otlp/collector/trace/v1:trace",
        "//vendor/go.opentelemetry.io/proto/otlp/trace/v1:trace",
        "//vendor/google.golang.org/genproto/googleapis/rpc/errdetails",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/credentials",
        "//vendor/google.golang.org/grpc/metadata",
        "//vendor/google.golang.org/grpc/status",
    ],
)
