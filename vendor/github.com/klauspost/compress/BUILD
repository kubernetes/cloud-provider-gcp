load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "compress",
    srcs = ["compressible.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/klauspost/compress",
    importpath = "github.com/klauspost/compress",
    visibility = ["//visibility:public"],
)
