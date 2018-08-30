load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["middleware.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/registry/middleware/repository",
    importpath = "github.com/docker/distribution/registry/middleware/repository",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/docker/distribution:go_default_library",
        "//vendor/github.com/docker/distribution/context:go_default_library",
    ],
)
