load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "encoding.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gonum.org/v1/gonum/graph/encoding",
    importpath = "gonum.org/v1/gonum/graph/encoding",
    visibility = ["//visibility:public"],
    deps = ["//vendor/gonum.org/v1/gonum/graph:go_default_library"],
)
