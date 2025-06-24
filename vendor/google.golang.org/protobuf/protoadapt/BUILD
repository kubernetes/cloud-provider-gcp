load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "protoadapt",
    srcs = ["convert.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/protobuf/protoadapt",
    importpath = "google.golang.org/protobuf/protoadapt",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/protobuf/proto",
        "//vendor/google.golang.org/protobuf/runtime/protoiface",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl",
    ],
)
