load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "fuzz.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/google/gofuzz",
    importpath = "github.com/google/gofuzz",
    visibility = ["//visibility:public"],
)
