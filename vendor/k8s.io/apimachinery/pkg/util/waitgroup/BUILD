load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "waitgroup.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apimachinery/pkg/util/waitgroup",
    importpath = "k8s.io/apimachinery/pkg/util/waitgroup",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["waitgroup_test.go"],
    embed = [":go_default_library"],
)
