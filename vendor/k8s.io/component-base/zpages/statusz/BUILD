load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "statusz",
    srcs = [
        "registry.go",
        "statusz.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/zpages/statusz",
    importpath = "k8s.io/component-base/zpages/statusz",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/version",
        "//vendor/k8s.io/component-base/compatibility",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/version",
        "//vendor/k8s.io/component-base/zpages/httputil",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
