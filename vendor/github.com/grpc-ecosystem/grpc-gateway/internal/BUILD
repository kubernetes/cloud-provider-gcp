load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["errors.pb.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/grpc-ecosystem/grpc-gateway/internal",
    importpath = "github.com/grpc-ecosystem/grpc-gateway/internal",
    visibility = ["//vendor/github.com/grpc-ecosystem/grpc-gateway:__subpackages__"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/github.com/golang/protobuf/ptypes/any:go_default_library",
    ],
)
