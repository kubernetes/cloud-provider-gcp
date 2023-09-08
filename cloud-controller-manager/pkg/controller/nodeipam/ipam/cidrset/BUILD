load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "cidrset",
    srcs = [
        "cidr_set.go",
        "metrics.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/cloud-controller-manager/pkg/controller/nodeipam/ipam/cidrset",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
    ],
)

go_test(
    name = "cidrset_test",
    srcs = ["cidr_set_test.go"],
    embed = [":cidrset"],
    deps = [
        "//vendor/k8s.io/component-base/metrics/testutil",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
