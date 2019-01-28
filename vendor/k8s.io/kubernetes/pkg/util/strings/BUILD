load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "escape.go",
        "line_delimiter.go",
        "strings.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/util/strings",
    importpath = "k8s.io/kubernetes/pkg/util/strings",
    visibility = ["//visibility:public"],
)
