load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["duration.pb.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/golang/protobuf/ptypes/duration",
    importpath = "github.com/golang/protobuf/ptypes/duration",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/golang/protobuf/proto:go_default_library"],
)
