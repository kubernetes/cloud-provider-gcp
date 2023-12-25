load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "types",
    srcs = [
        "helpers.go",
        "types.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/cmd/client-gen/types",
    importpath = "k8s.io/code-generator/cmd/client-gen/types",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/gengo/namer"],
)
