load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "otlpgrpc",
    srcs = [
        "connection.go",
        "doc.go",
        "driver.go",
        "options.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp/otlpgrpc",
    importpath = "go.opentelemetry.io/otel/exporters/otlp/otlpgrpc",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/internal/otlpconfig",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/internal/transform",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric",
        "//vendor/go.opentelemetry.io/otel/sdk/trace",
        "//vendor/go.opentelemetry.io/proto/otlp/collector/metrics/v1:metrics",
        "//vendor/go.opentelemetry.io/proto/otlp/collector/trace/v1:trace",
        "//vendor/go.opentelemetry.io/proto/otlp/metrics/v1:metrics",
        "//vendor/go.opentelemetry.io/proto/otlp/trace/v1:trace",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/credentials",
        "//vendor/google.golang.org/grpc/encoding/gzip",
        "//vendor/google.golang.org/grpc/metadata",
    ],
)
