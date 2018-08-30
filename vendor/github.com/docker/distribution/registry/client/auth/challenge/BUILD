load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "addr.go",
        "authchallenge.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/registry/client/auth/challenge",
    importpath = "github.com/docker/distribution/registry/client/auth/challenge",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["authchallenge_test.go"],
    embed = [":go_default_library"],
)
