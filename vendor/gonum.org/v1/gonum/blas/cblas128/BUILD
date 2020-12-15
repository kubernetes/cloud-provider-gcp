load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "cblas128.go",
        "conv.go",
        "conv_hermitian.go",
        "conv_symmetric.go",
        "doc.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gonum.org/v1/gonum/blas/cblas128",
    importpath = "gonum.org/v1/gonum/blas/cblas128",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/gonum.org/v1/gonum/blas:go_default_library",
        "//vendor/gonum.org/v1/gonum/blas/gonum:go_default_library",
    ],
)
