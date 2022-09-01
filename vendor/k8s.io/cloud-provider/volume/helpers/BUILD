load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "helpers",
    srcs = [
        "rounding.go",
        "zones.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/cloud-provider/volume/helpers",
    importpath = "k8s.io/cloud-provider/volume/helpers",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/resource",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/cloud-provider/volume",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
