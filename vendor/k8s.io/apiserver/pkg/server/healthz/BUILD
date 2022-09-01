load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "healthz",
    srcs = [
        "doc.go",
        "healthz.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/server/healthz",
    importpath = "k8s.io/apiserver/pkg/server/healthz",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/endpoints/metrics",
        "//vendor/k8s.io/apiserver/pkg/server/httplog",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
