load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "otelgrpc",
    srcs = [
        "grpctrace.go",
        "interceptor.go",
        "interceptorinfo.go",
        "semconv.go",
        "version.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc",
    importpath = "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto",
        "//vendor/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc/internal",
        "//vendor/go.opentelemetry.io/otel",
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/baggage",
        "//vendor/go.opentelemetry.io/otel/codes",
        "//vendor/go.opentelemetry.io/otel/propagation",
        "//vendor/go.opentelemetry.io/otel/semconv/v1.12.0:v1_12_0",
        "//vendor/go.opentelemetry.io/otel/trace",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/metadata",
        "//vendor/google.golang.org/grpc/peer",
        "//vendor/google.golang.org/grpc/status",
    ],
)
