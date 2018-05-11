load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "ratelimit.go",
        "reader.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/juju/ratelimit",
    importpath = "github.com/juju/ratelimit",
    visibility = ["//visibility:public"],
)
