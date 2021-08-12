load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
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
        "//vendor/go.opentelemetry.io/otel:go_default_library",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp:go_default_library",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/internal/otlpconfig:go_default_library",
        "//vendor/go.opentelemetry.io/otel/exporters/otlp/internal/transform:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/trace:go_default_library",
        "//vendor/go.opentelemetry.io/proto/otlp/collector/metrics/v1:go_default_library",
        "//vendor/go.opentelemetry.io/proto/otlp/collector/trace/v1:go_default_library",
        "//vendor/go.opentelemetry.io/proto/otlp/metrics/v1:go_default_library",
        "//vendor/go.opentelemetry.io/proto/otlp/trace/v1:go_default_library",
        "//vendor/google.golang.org/grpc:go_default_library",
        "//vendor/google.golang.org/grpc/credentials:go_default_library",
        "//vendor/google.golang.org/grpc/encoding/gzip:go_default_library",
        "//vendor/google.golang.org/grpc/metadata:go_default_library",
    ],
)
