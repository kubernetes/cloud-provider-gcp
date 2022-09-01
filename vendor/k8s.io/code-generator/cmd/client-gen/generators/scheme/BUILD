load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "scheme",
    srcs = ["generator_for_scheme.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/code-generator/cmd/client-gen/generators/scheme",
    importpath = "k8s.io/code-generator/cmd/client-gen/generators/scheme",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/code-generator/cmd/client-gen/path",
        "//vendor/k8s.io/code-generator/cmd/client-gen/types",
        "//vendor/k8s.io/gengo/generator",
        "//vendor/k8s.io/gengo/namer",
        "//vendor/k8s.io/gengo/types",
    ],
)
