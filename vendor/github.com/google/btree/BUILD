load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["btree.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/google/btree",
    importpath = "github.com/google/btree",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["btree_test.go"],
    embed = [":go_default_library"],
)
