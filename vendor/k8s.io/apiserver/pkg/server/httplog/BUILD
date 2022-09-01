load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "httplog",
    srcs = [
        "doc.go",
        "httplog.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/server/httplog",
    importpath = "k8s.io/apiserver/pkg/server/httplog",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/endpoints/metrics",
        "//vendor/k8s.io/apiserver/pkg/endpoints/request",
        "//vendor/k8s.io/apiserver/pkg/endpoints/responsewriter",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
