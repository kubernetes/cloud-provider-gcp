load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metrics",
    srcs = ["metrics.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/etcd3/metrics",
    importpath = "k8s.io/apiserver/pkg/storage/etcd3/metrics",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apiserver/pkg/features",
        "//vendor/k8s.io/apiserver/pkg/storage",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
