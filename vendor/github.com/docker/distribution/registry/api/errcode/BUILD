load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "errors.go",
        "handler.go",
        "register.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/registry/api/errcode",
    importpath = "github.com/docker/distribution/registry/api/errcode",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["errors_test.go"],
    embed = [":go_default_library"],
)
