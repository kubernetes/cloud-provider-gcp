load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "parser",
    srcs = [
        "doc.go",
        "parse.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/gengo/parser",
    importpath = "k8s.io/gengo/parser",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/gengo/types",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
