load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "trace",
    srcs = ["trace.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/utils/trace",
    importpath = "k8s.io/utils/trace",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/klog/v2:klog"],
)
