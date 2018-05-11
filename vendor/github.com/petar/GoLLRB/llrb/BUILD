load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "avgvar.go",
        "iterator.go",
        "llrb.go",
        "llrb-stats.go",
        "util.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/petar/GoLLRB/llrb",
    importpath = "github.com/petar/GoLLRB/llrb",
    visibility = ["//visibility:public"],
)
