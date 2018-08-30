load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/cmd/digest",
    importpath = "github.com/docker/distribution/cmd/digest",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/github.com/docker/distribution/version:go_default_library",
        "//vendor/github.com/opencontainers/go-digest:go_default_library",
    ],
)

go_binary(
    name = "digest",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
