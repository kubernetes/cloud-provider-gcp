load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["testsuites.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/registry/storage/driver/testsuites",
    importpath = "github.com/docker/distribution/registry/storage/driver/testsuites",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/docker/distribution/context:go_default_library",
        "//vendor/github.com/docker/distribution/registry/storage/driver:go_default_library",
        "@in_gopkg_check_v1//:go_default_library",
    ],
)
