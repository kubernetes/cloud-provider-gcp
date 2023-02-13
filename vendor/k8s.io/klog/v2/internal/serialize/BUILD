load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "serialize",
    srcs = ["keyvalues.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/klog/v2/internal/serialize",
    importpath = "k8s.io/klog/v2/internal/serialize",
    visibility = ["//vendor/k8s.io/klog/v2:__subpackages__"],
    deps = ["//vendor/github.com/go-logr/logr"],
)
