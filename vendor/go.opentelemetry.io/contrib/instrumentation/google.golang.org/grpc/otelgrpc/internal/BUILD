load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = ["parse.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc/internal",
    importpath = "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc/internal",
    visibility = ["//vendor/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc:__subpackages__"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/semconv/v1.12.0:v1_12_0",
    ],
)
