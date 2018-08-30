load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["optional.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/internal/optional",
    importpath = "cloud.google.com/go/internal/optional",
    visibility = ["//vendor/cloud.google.com/go:__subpackages__"],
)

go_test(
    name = "go_default_test",
    srcs = ["optional_test.go"],
    embed = [":go_default_library"],
)
