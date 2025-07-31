load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cbor",
    srcs = [
        "bytestring.go",
        "cache.go",
        "common.go",
        "decode.go",
        "diagnose.go",
        "doc.go",
        "encode.go",
        "encode_map.go",
        "encode_map_go117.go",
        "simplevalue.go",
        "stream.go",
        "structfields.go",
        "tag.go",
        "valid.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/fxamacker/cbor/v2",
    importpath = "github.com/fxamacker/cbor/v2",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/x448/float16"],
)
