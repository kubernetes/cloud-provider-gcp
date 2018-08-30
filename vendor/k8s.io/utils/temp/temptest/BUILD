load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "dir.go",
        "doc.go",
        "file.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/utils/temp/temptest",
    importpath = "k8s.io/utils/temp/temptest",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/utils/temp:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "dir_test.go",
        "example_test.go",
        "file_test.go",
    ],
    embed = [":go_default_library"],
    deps = ["//vendor/k8s.io/utils/temp:go_default_library"],
)
