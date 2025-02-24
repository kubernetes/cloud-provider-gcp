load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "args",
    srcs = ["args.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/cmd/lister-gen/args",
    importpath = "k8s.io/code-generator/cmd/lister-gen/args",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/spf13/pflag"],
)
