load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "filterlatency",
    srcs = ["filterlatency.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/filterlatency",
    importpath = "k8s.io/apiserver/pkg/endpoints/filterlatency",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/endpoints/metrics",
        "//vendor/k8s.io/apiserver/pkg/endpoints/request",
        "//vendor/k8s.io/apiserver/pkg/server/httplog",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/clock",
    ],
)
