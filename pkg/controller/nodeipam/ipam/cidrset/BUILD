load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "cidr_set.go",
        "metrics.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/ipam/cidrset",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/component-base/metrics:go_default_library",
        "//vendor/k8s.io/component-base/metrics/legacyregistry:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["cidr_set_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/component-base/metrics/testutil:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
    ],
)
