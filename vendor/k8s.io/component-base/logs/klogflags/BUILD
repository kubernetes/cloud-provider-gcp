load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "klogflags",
    srcs = ["klogflags.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/logs/klogflags",
    importpath = "k8s.io/component-base/logs/klogflags",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/klog/v2:klog"],
)
