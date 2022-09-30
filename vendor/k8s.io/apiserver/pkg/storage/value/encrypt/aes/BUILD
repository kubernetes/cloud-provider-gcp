load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "aes",
    srcs = ["aes.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/value/encrypt/aes",
    importpath = "k8s.io/apiserver/pkg/storage/value/encrypt/aes",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/apiserver/pkg/storage/value"],
)
