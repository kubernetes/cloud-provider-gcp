load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["storagemiddleware.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/registry/storage/driver/middleware",
    importpath = "github.com/docker/distribution/registry/storage/driver/middleware",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/docker/distribution/registry/storage/driver:go_default_library"],
)
