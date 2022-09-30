load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "descriptor",
    srcs = ["descriptor.pb.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/golang/protobuf/protoc-gen-go/descriptor",
    importpath = "github.com/golang/protobuf/protoc-gen-go/descriptor",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl",
        "//vendor/google.golang.org/protobuf/types/descriptorpb",
    ],
)
