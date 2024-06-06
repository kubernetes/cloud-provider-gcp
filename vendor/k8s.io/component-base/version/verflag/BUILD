load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "verflag",
    srcs = ["verflag.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/version/verflag",
    importpath = "k8s.io/component-base/version/verflag",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/component-base/version",
    ],
)
