load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "snapref",
    srcs = [
        "decode.go",
        "decode_other.go",
        "encode.go",
        "encode_other.go",
        "snappy.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/klauspost/compress/internal/snapref",
    importpath = "github.com/klauspost/compress/internal/snapref",
    visibility = ["//vendor/github.com/klauspost/compress:__subpackages__"],
)
