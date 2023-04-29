load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "logs",
    srcs = ["logs.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/logs",
    importpath = "k8s.io/component-base/logs",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/component-base/logs/api/v1:api",
        "//vendor/k8s.io/component-base/logs/internal/setverbositylevel",
        "//vendor/k8s.io/component-base/logs/klogflags",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
