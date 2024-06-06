load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "dynamicpb",
    srcs = [
        "dynamic.go",
        "types.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/protobuf/types/dynamicpb",
    importpath = "google.golang.org/protobuf/types/dynamicpb",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/protobuf/internal/errors",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
        "//vendor/google.golang.org/protobuf/reflect/protoregistry",
        "//vendor/google.golang.org/protobuf/runtime/protoiface",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl",
    ],
)
