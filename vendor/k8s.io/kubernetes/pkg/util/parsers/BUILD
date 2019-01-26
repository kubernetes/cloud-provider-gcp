load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["parsers.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/util/parsers",
    importpath = "k8s.io/kubernetes/pkg/util/parsers",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/docker/distribution/reference:go_default_library"],
)
