load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "graph.go",
        "multigraph.go",
        "nodes_edges.go",
        "undirect.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gonum.org/v1/gonum/graph",
    importpath = "gonum.org/v1/gonum/graph",
    visibility = ["//visibility:public"],
)
