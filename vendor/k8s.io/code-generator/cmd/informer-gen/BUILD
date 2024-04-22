load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "informer-gen_lib",
    srcs = ["main.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/cmd/informer-gen",
    importpath = "k8s.io/code-generator/cmd/informer-gen",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/code-generator/cmd/informer-gen/args",
        "//vendor/k8s.io/code-generator/cmd/informer-gen/generators",
        "//vendor/k8s.io/code-generator/pkg/util",
        "//vendor/k8s.io/gengo/v2:gengo",
        "//vendor/k8s.io/gengo/v2/generator",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)

go_binary(
    name = "informer-gen",
    embed = [":informer-gen_lib"],
    visibility = ["//visibility:public"],
)
