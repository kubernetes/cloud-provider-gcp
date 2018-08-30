load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["combo.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/gogo/protobuf/protoc-gen-combo",
    importpath = "github.com/gogo/protobuf/protoc-gen-combo",
    visibility = ["//visibility:private"],
    deps = ["//vendor/github.com/gogo/protobuf/version:go_default_library"],
)

go_binary(
    name = "protoc-gen-combo",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
