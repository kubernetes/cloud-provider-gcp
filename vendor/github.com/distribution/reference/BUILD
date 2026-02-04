load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "reference",
    srcs = [
        "helpers.go",
        "normalize.go",
        "reference.go",
        "regexp.go",
        "sort.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/distribution/reference",
    importpath = "github.com/distribution/reference",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/opencontainers/go-digest"],
)
