load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["checks.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/health/checks",
    importpath = "github.com/docker/distribution/health/checks",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/docker/distribution/health:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["checks_test.go"],
    embed = [":go_default_library"],
)
