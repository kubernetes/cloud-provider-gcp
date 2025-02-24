load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "stdr",
    srcs = ["stdr.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/go-logr/stdr",
    importpath = "github.com/go-logr/stdr",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/go-logr/logr",
        "//vendor/github.com/go-logr/logr/funcr",
    ],
)
