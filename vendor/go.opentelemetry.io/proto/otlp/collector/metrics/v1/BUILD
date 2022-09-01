load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metrics",
    srcs = [
        "metrics_service.pb.go",
        "metrics_service.pb.gw.go",
        "metrics_service_grpc.pb.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/proto/otlp/collector/metrics/v1",
    importpath = "go.opentelemetry.io/proto/otlp/collector/metrics/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/protobuf/descriptor",
        "//vendor/github.com/golang/protobuf/proto",
        "//vendor/github.com/grpc-ecosystem/grpc-gateway/runtime",
        "//vendor/github.com/grpc-ecosystem/grpc-gateway/utilities",
        "//vendor/go.opentelemetry.io/proto/otlp/metrics/v1:metrics",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/grpclog",
        "//vendor/google.golang.org/grpc/metadata",
        "//vendor/google.golang.org/grpc/status",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl",
    ],
)
