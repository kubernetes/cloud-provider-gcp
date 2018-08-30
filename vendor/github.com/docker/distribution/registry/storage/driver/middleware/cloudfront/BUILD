load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["middleware.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/registry/storage/driver/middleware/cloudfront",
    importpath = "github.com/docker/distribution/registry/storage/driver/middleware/cloudfront",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/docker/distribution/context:go_default_library",
        "//vendor/github.com/docker/distribution/registry/storage/driver:go_default_library",
        "//vendor/github.com/docker/distribution/registry/storage/driver/middleware:go_default_library",
        "@com_github_aws_aws_sdk_go//service/cloudfront/sign:go_default_library",
    ],
)
