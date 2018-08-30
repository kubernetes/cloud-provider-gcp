load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["grpc.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/golang/protobuf/protoc-gen-go/grpc",
    importpath = "github.com/golang/protobuf/protoc-gen-go/grpc",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/protobuf/protoc-gen-go/descriptor:go_default_library",
        "//vendor/github.com/golang/protobuf/protoc-gen-go/generator:go_default_library",
    ],
)
