load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "klog.go",
        "klog_file.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/klog",
    importpath = "k8s.io/klog",
    visibility = ["//visibility:public"],
)
