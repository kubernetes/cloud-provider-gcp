load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "keys.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/golang.org/x/crypto/ssh/testdata",
    importpath = "golang.org/x/crypto/ssh/testdata",
    visibility = ["//visibility:public"],
)
