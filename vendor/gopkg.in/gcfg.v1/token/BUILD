load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "token",
    srcs = [
        "position.go",
        "serialize.go",
        "token.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gopkg.in/gcfg.v1/token",
    importpath = "gopkg.in/gcfg.v1/token",
    visibility = ["//visibility:public"],
)
