load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cloud-provider",
    srcs = [
        "cloud.go",
        "doc.go",
        "plugins.go",
        "ports.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/cloud-provider",
    importpath = "k8s.io/cloud-provider",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
