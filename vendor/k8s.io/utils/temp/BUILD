load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "dir.go",
        "doc.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/utils/temp",
    importpath = "k8s.io/utils/temp",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["dir_test.go"],
    embed = [":go_default_library"],
)
