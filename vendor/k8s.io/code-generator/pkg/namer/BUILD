load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "namer",
    srcs = ["tag-override.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/pkg/namer",
    importpath = "k8s.io/code-generator/pkg/namer",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/gengo/namer",
        "//vendor/k8s.io/gengo/types",
    ],
)
