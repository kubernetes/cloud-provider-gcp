load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fse",
    srcs = [
        "bitreader.go",
        "bitwriter.go",
        "bytereader.go",
        "compress.go",
        "decompress.go",
        "fse.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/klauspost/compress/fse",
    importpath = "github.com/klauspost/compress/fse",
    visibility = ["//visibility:public"],
)
