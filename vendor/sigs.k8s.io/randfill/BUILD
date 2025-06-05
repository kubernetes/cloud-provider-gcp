load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "randfill",
    srcs = ["randfill.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/randfill",
    importpath = "sigs.k8s.io/randfill",
    visibility = ["//visibility:public"],
    deps = ["//vendor/sigs.k8s.io/randfill/bytesource"],
)
