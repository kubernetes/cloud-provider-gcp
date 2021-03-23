load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "lapack.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gonum.org/v1/gonum/lapack",
    importpath = "gonum.org/v1/gonum/lapack",
    visibility = ["//visibility:public"],
    deps = ["//vendor/gonum.org/v1/gonum/blas:go_default_library"],
)
