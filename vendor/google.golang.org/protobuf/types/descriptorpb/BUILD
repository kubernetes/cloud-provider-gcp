load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "descriptorpb",
    srcs = ["descriptor.pb.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/protobuf/types/descriptorpb",
    importpath = "google.golang.org/protobuf/types/descriptorpb",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl",
    ],
)
