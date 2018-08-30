load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["civil.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/civil",
    importpath = "cloud.google.com/go/civil",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["civil_test.go"],
    embed = [":go_default_library"],
    deps = ["@com_github_google_go_cmp//cmp:go_default_library"],
)
