load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["iterator.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/google.golang.org/api/iterator",
    importpath = "google.golang.org/api/iterator",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "examples_test.go",
        "iterator_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/iterator/testing:go_default_library",
    ],
)
