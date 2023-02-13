load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "hpack",
    srcs = [
        "encode.go",
        "hpack.go",
        "huffman.go",
        "static_table.go",
        "tables.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/net/http2/hpack",
    importpath = "golang.org/x/net/http2/hpack",
    visibility = ["//visibility:public"],
)
