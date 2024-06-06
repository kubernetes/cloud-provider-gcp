load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "clock",
    srcs = ["clock.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/klog/v2/internal/clock",
    importpath = "k8s.io/klog/v2/internal/clock",
    visibility = ["//vendor/k8s.io/klog/v2:__subpackages__"],
)
