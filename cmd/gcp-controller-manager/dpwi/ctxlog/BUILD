load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "ctxlog",
    srcs = ["ctxlog.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/ctxlog",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
