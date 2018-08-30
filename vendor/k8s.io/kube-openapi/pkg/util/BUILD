load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "trie.go",
        "util.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kube-openapi/pkg/util",
    importpath = "k8s.io/kube-openapi/pkg/util",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["util_test.go"],
    embed = [":go_default_library"],
)
