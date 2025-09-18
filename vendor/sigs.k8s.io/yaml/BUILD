load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "yaml",
    srcs = [
        "fields.go",
        "yaml.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/yaml",
    importpath = "sigs.k8s.io/yaml",
    visibility = ["//visibility:public"],
    deps = ["//vendor/go.yaml.in/yaml/v2:yaml"],
)
