load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "proto",
    srcs = ["proto.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/encoding/proto",
    importpath = "google.golang.org/grpc/encoding/proto",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc/encoding",
        "//vendor/google.golang.org/protobuf/proto",
        "//vendor/google.golang.org/protobuf/protoadapt",
    ],
)
