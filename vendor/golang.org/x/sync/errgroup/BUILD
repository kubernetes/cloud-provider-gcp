load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "errgroup",
    srcs = [
        "errgroup.go",
        "go120.go",
        "pre_go120.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/sync/errgroup",
    importpath = "golang.org/x/sync/errgroup",
    visibility = ["//visibility:public"],
)
