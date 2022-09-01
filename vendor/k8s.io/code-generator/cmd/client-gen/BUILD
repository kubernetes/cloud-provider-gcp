load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "client-gen_lib",
    srcs = ["main.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/cmd/client-gen",
    importpath = "k8s.io/code-generator/cmd/client-gen",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/code-generator/cmd/client-gen/args",
        "//vendor/k8s.io/code-generator/cmd/client-gen/generators",
        "//vendor/k8s.io/code-generator/pkg/util",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)

go_binary(
    name = "client-gen",
    embed = [":client-gen_lib"],
    visibility = ["//visibility:public"],
)
