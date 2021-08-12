load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["descriptor.pb.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/golang/protobuf/protoc-gen-go/descriptor",
    importpath = "github.com/golang/protobuf/protoc-gen-go/descriptor",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/protobuf/reflect/protoreflect:go_default_library",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl:go_default_library",
        "//vendor/google.golang.org/protobuf/types/descriptorpb:go_default_library",
    ],
)
