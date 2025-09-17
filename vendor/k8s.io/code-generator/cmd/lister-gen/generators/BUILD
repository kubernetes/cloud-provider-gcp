load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "generators",
    srcs = [
        "expansion.go",
        "lister.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/cmd/lister-gen/generators",
    importpath = "k8s.io/code-generator/cmd/lister-gen/generators",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/code-generator/cmd/client-gen/generators/util",
        "//vendor/k8s.io/code-generator/cmd/client-gen/types",
        "//vendor/k8s.io/code-generator/cmd/lister-gen/args",
        "//vendor/k8s.io/code-generator/pkg/util",
        "//vendor/k8s.io/gengo/v2:gengo",
        "//vendor/k8s.io/gengo/v2/generator",
        "//vendor/k8s.io/gengo/v2/namer",
        "//vendor/k8s.io/gengo/v2/types",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
