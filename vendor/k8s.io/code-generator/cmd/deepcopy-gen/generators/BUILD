load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "generators",
    srcs = ["deepcopy.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/cmd/deepcopy-gen/generators",
    importpath = "k8s.io/code-generator/cmd/deepcopy-gen/generators",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/code-generator/cmd/deepcopy-gen/args",
        "//vendor/k8s.io/code-generator/pkg/util",
        "//vendor/k8s.io/gengo/v2:gengo",
        "//vendor/k8s.io/gengo/v2/generator",
        "//vendor/k8s.io/gengo/v2/namer",
        "//vendor/k8s.io/gengo/v2/types",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
