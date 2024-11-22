load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "feature",
    srcs = ["feature_support_checker.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/feature",
    importpath = "k8s.io/apiserver/pkg/storage/feature",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.etcd.io/etcd/client/v3:client",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/version",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/storage",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/ptr",
    ],
)
