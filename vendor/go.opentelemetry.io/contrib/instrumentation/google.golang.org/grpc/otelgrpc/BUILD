load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "grpctrace.go",
        "interceptor.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc",
    importpath = "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/go.opentelemetry.io/contrib:go_default_library",
        "//vendor/go.opentelemetry.io/otel:go_default_library",
        "//vendor/go.opentelemetry.io/otel/attribute:go_default_library",
        "//vendor/go.opentelemetry.io/otel/baggage:go_default_library",
        "//vendor/go.opentelemetry.io/otel/codes:go_default_library",
        "//vendor/go.opentelemetry.io/otel/propagation:go_default_library",
        "//vendor/go.opentelemetry.io/otel/semconv:go_default_library",
        "//vendor/go.opentelemetry.io/otel/trace:go_default_library",
        "//vendor/google.golang.org/grpc:go_default_library",
        "//vendor/google.golang.org/grpc/codes:go_default_library",
        "//vendor/google.golang.org/grpc/metadata:go_default_library",
        "//vendor/google.golang.org/grpc/peer:go_default_library",
        "//vendor/google.golang.org/grpc/status:go_default_library",
    ],
)
