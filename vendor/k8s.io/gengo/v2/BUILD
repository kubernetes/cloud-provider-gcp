load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "gengo",
    srcs = [
        "comments.go",
        "execute.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/gengo/v2",
    importpath = "k8s.io/gengo/v2",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/gengo/v2/generator",
        "//vendor/k8s.io/gengo/v2/namer",
        "//vendor/k8s.io/gengo/v2/parser",
    ],
)
