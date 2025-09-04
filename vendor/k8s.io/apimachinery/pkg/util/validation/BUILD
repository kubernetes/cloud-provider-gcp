load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "validation",
    srcs = [
        "ip.go",
        "validation.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/util/validation",
    importpath = "k8s.io/apimachinery/pkg/util/validation",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/net",
    ],
)
