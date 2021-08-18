load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "pattern.go",
        "readerfactory.go",
        "trie.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/grpc-ecosystem/grpc-gateway/utilities",
    importpath = "github.com/grpc-ecosystem/grpc-gateway/utilities",
    visibility = ["//visibility:public"],
)
