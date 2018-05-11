load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "lru.go",
        "lru_interface.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/hashicorp/golang-lru/simplelru",
    importpath = "github.com/hashicorp/golang-lru/simplelru",
    visibility = ["//visibility:public"],
)
