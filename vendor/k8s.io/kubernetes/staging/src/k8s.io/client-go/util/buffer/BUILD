load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["ring_growing.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/client-go/util/buffer",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/client-go/util/buffer",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["ring_growing_test.go"],
    embed = [":go_default_library"],
    deps = ["@com_github_stretchr_testify//assert:go_default_library"],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
