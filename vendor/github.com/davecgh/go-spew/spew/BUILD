load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "bypass.go",
        "common.go",
        "config.go",
        "doc.go",
        "dump.go",
        "format.go",
        "spew.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/davecgh/go-spew/spew",
    importpath = "github.com/davecgh/go-spew/spew",
    visibility = ["//visibility:public"],
)
