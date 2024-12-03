load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "huff0",
    srcs = [
        "bitreader.go",
        "bitwriter.go",
        "compress.go",
        "decompress.go",
        "decompress_amd64.go",
        "decompress_amd64.s",
        "decompress_generic.go",
        "huff0.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/klauspost/compress/huff0",
    importpath = "github.com/klauspost/compress/huff0",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/klauspost/compress/fse",
    ] + select({
        "@io_bazel_rules_go//go/platform:amd64": [
            "//vendor/github.com/klauspost/compress/internal/cpuinfo",
        ],
        "//conditions:default": [],
    }),
)
