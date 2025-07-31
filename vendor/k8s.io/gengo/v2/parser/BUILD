load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "parser",
    srcs = [
        "doc.go",
        "parse.go",
        "parse_122.go",
        "parse_pre_122.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/gengo/v2/parser",
    importpath = "k8s.io/gengo/v2/parser",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/tools/go/packages",
        "//vendor/k8s.io/gengo/v2/types",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
