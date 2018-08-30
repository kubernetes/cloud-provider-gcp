load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "markdown.go",
        "normalizer.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/util/normalizer",
    importpath = "k8s.io/kubernetes/pkg/util/normalizer",
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_makenowjust_heredoc//:go_default_library",
        "@com_github_russross_blackfriday//:go_default_library",
    ],
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
