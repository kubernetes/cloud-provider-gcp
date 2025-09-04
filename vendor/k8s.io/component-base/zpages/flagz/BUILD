load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "flagz",
    srcs = [
        "flagreader.go",
        "flagz.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/zpages/flagz",
    importpath = "k8s.io/component-base/zpages/flagz",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/component-base/cli/flag",
        "//vendor/k8s.io/component-base/zpages/httputil",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
