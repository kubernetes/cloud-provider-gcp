load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "util",
    srcs = ["controller_utils.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/util",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/tools/record",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
