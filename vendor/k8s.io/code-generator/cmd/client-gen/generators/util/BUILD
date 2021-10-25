load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "gvpackages.go",
        "tags.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/cmd/client-gen/generators/util",
    importpath = "k8s.io/code-generator/cmd/client-gen/generators/util",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/gengo/types:go_default_library"],
)
