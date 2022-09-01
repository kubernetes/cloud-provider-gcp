load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "generators",
    srcs = [
        "packages.go",
        "register_external.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/cmd/register-gen/generators",
    importpath = "k8s.io/code-generator/cmd/register-gen/generators",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/code-generator/cmd/client-gen/types",
        "//vendor/k8s.io/gengo/args",
        "//vendor/k8s.io/gengo/generator",
        "//vendor/k8s.io/gengo/namer",
        "//vendor/k8s.io/gengo/types",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
