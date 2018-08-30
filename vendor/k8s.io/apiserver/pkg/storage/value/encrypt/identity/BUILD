load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["identity.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apiserver/pkg/storage/value/encrypt/identity",
    importpath = "k8s.io/apiserver/pkg/storage/value/encrypt/identity",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/apiserver/pkg/storage/value:go_default_library"],
)
