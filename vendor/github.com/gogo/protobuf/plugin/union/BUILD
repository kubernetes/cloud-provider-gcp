load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "union.go",
        "uniontest.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/gogo/protobuf/plugin/union",
    importpath = "github.com/gogo/protobuf/plugin/union",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/gogo/protobuf/gogoproto:go_default_library",
        "//vendor/github.com/gogo/protobuf/plugin/testgen:go_default_library",
        "//vendor/github.com/gogo/protobuf/protoc-gen-gogo/generator:go_default_library",
    ],
)
