load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "btree.go",
        "debug.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/internal/btree",
    importpath = "cloud.google.com/go/internal/btree",
    visibility = ["//vendor/cloud.google.com/go:__subpackages__"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "benchmarks_test.go",
        "btree_test.go",
    ],
    embed = [":go_default_library"],
    deps = ["@com_github_google_go_cmp//cmp:go_default_library"],
)
