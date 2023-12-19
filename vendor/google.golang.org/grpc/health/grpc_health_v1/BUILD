load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "grpc_health_v1",
    srcs = [
        "health.pb.go",
        "health_grpc.pb.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/health/grpc_health_v1",
    importpath = "google.golang.org/grpc/health/grpc_health_v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/status",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl",
    ],
)
