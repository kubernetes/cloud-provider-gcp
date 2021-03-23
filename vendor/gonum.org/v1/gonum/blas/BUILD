load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "blas.go",
        "doc.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gonum.org/v1/gonum/blas",
    importpath = "gonum.org/v1/gonum/blas",
    visibility = ["//visibility:public"],
)
