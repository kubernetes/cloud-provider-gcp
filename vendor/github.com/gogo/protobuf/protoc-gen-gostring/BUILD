load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/gogo/protobuf/protoc-gen-gostring",
    importpath = "github.com/gogo/protobuf/protoc-gen-gostring",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/github.com/gogo/protobuf/plugin/gostring:go_default_library",
        "//vendor/github.com/gogo/protobuf/vanity/command:go_default_library",
    ],
)

go_binary(
    name = "protoc-gen-gostring",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
