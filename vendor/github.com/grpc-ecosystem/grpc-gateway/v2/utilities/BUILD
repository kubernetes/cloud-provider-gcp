load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "utilities",
    srcs = [
        "doc.go",
        "pattern.go",
        "readerfactory.go",
        "trie.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/grpc-ecosystem/grpc-gateway/v2/utilities",
    importpath = "github.com/grpc-ecosystem/grpc-gateway/v2/utilities",
    visibility = ["//visibility:public"],
)
