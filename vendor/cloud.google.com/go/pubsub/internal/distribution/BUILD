load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["distribution.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/pubsub/internal/distribution",
    importpath = "cloud.google.com/go/pubsub/internal/distribution",
    visibility = ["//vendor/cloud.google.com/go/pubsub:__subpackages__"],
)

go_test(
    name = "go_default_test",
    srcs = ["distribution_test.go"],
    embed = [":go_default_library"],
)
