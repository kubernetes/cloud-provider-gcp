load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["log.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/prometheus/common/promlog",
    importpath = "github.com/prometheus/common/promlog",
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_go_kit_kit//log:go_default_library",
        "@com_github_go_kit_kit//log/level:go_default_library",
        "@com_github_pkg_errors//:go_default_library",
    ],
)
