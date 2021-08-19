load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["field_mask.pb.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/protobuf/types/known/fieldmaskpb",
    importpath = "google.golang.org/protobuf/types/known/fieldmaskpb",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/protobuf/proto:go_default_library",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect:go_default_library",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl:go_default_library",
    ],
)
