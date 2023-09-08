load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "sync",
    srcs = ["sync.go"],
    importpath = "k8s.io/cloud-provider-gcp/cloud-controller-manager/pkg/controller/nodeipam/ipam/sync",
    visibility = ["//visibility:public"],
    deps = [
        "//cloud-controller-manager/pkg/controller/nodeipam/ipam/cidrset",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)

go_test(
    name = "sync_test",
    srcs = ["sync_test.go"],
    embed = [":sync"],
    deps = [
        "//cloud-controller-manager/pkg/controller/nodeipam/ipam/cidrset",
        "//cloud-controller-manager/pkg/controller/nodeipam/ipam/test",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
