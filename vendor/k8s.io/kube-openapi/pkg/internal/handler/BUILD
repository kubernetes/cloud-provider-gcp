load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "handler",
    srcs = ["handler_cache.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kube-openapi/pkg/internal/handler",
    importpath = "k8s.io/kube-openapi/pkg/internal/handler",
    visibility = ["//vendor/k8s.io/kube-openapi/pkg:__subpackages__"],
)
