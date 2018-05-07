load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["pointer.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/go-openapi/jsonpointer",
    importpath = "github.com/go-openapi/jsonpointer",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/go-openapi/swag:go_default_library"],
)
