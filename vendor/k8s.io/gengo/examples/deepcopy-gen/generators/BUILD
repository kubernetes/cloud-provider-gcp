load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "generators",
    srcs = ["deepcopy.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/gengo/examples/deepcopy-gen/generators",
    importpath = "k8s.io/gengo/examples/deepcopy-gen/generators",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/gengo/args",
        "//vendor/k8s.io/gengo/examples/set-gen/sets",
        "//vendor/k8s.io/gengo/generator",
        "//vendor/k8s.io/gengo/namer",
        "//vendor/k8s.io/gengo/types",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
