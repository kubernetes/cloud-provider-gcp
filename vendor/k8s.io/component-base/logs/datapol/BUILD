load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "datapol.go",
        "externaltypes.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/logs/datapol",
    importpath = "k8s.io/component-base/logs/datapol",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/klog/v2:go_default_library"],
)
