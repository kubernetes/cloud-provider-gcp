load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "args",
    srcs = ["args.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/gengo/args",
    importpath = "k8s.io/gengo/args",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/gengo/generator",
        "//vendor/k8s.io/gengo/namer",
        "//vendor/k8s.io/gengo/parser",
        "//vendor/k8s.io/gengo/types",
    ],
)
