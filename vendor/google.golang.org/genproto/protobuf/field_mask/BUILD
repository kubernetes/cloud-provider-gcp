load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["field_mask.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/genproto/protobuf/field_mask",
    importpath = "google.golang.org/genproto/protobuf/field_mask",
    visibility = ["//visibility:public"],
    deps = ["//vendor/google.golang.org/protobuf/types/known/fieldmaskpb:go_default_library"],
)
