load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "trace",
    srcs = [
        "trace_config.pb.go",
        "trace_service.pb.go",
        "trace_service.pb.gw.go",
        "trace_service_grpc.pb.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/proto/otlp/collector/trace/v1",
    importpath = "go.opentelemetry.io/proto/otlp/collector/trace/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/protobuf/descriptor",
        "//vendor/github.com/golang/protobuf/proto",
        "//vendor/github.com/grpc-ecosystem/grpc-gateway/runtime",
        "//vendor/github.com/grpc-ecosystem/grpc-gateway/utilities",
        "//vendor/go.opentelemetry.io/proto/otlp/trace/v1:trace",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/grpclog",
        "//vendor/google.golang.org/grpc/metadata",
        "//vendor/google.golang.org/grpc/status",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl",
    ],
)
