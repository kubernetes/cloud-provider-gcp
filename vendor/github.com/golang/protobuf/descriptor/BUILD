load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "descriptor",
    srcs = ["descriptor.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/golang/protobuf/descriptor",
    importpath = "github.com/golang/protobuf/descriptor",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto",
        "//vendor/github.com/golang/protobuf/protoc-gen-go/descriptor",
        "//vendor/google.golang.org/protobuf/reflect/protodesc",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl",
    ],
)
