load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "args",
    srcs = [
        "args.go",
        "gvpackages.go",
        "gvtype.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/cmd/client-gen/args",
    importpath = "k8s.io/code-generator/cmd/client-gen/args",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/code-generator/cmd/client-gen/generators/util",
        "//vendor/k8s.io/code-generator/cmd/client-gen/types",
    ],
)
