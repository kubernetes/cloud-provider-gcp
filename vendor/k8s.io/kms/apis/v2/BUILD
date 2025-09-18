load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "apis",
    srcs = [
        "api.pb.go",
        "api_grpc.pb.go",
        "v2.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kms/apis/v2",
    importpath = "k8s.io/kms/apis/v2",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/status",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl",
    ],
)
