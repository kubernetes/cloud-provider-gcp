load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "edges.go",
        "lines.go",
        "nodes.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gonum.org/v1/gonum/graph/iterator",
    importpath = "gonum.org/v1/gonum/graph/iterator",
    visibility = ["//visibility:public"],
    deps = ["//vendor/gonum.org/v1/gonum/graph:go_default_library"],
)
