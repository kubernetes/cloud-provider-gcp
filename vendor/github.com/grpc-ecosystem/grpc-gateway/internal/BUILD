load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = ["errors.pb.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/grpc-ecosystem/grpc-gateway/internal",
    importpath = "github.com/grpc-ecosystem/grpc-gateway/internal",
    visibility = ["//vendor/github.com/grpc-ecosystem/grpc-gateway:__subpackages__"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto",
        "//vendor/github.com/golang/protobuf/ptypes/any",
    ],
)
