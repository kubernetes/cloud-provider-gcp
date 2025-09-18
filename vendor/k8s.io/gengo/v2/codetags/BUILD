load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "codetags",
    srcs = [
        "extractor.go",
        "parser.go",
        "scanner.go",
        "types.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/gengo/v2/codetags",
    importpath = "k8s.io/gengo/v2/codetags",
    visibility = ["//visibility:public"],
)
