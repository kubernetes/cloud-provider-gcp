load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "slogr",
    srcs = [
        "sloghandler.go",
        "slogr.go",
        "slogsink.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/go-logr/logr/slogr",
    importpath = "github.com/go-logr/logr/slogr",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/go-logr/logr"],
)
