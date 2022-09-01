load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "node",
    srcs = ["node.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/util/node",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
