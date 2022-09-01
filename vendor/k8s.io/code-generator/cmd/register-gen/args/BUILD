load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "args",
    srcs = ["args.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/cmd/register-gen/args",
    importpath = "k8s.io/code-generator/cmd/register-gen/args",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/gengo/args"],
)
