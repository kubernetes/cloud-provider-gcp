load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "featuregate",
    srcs = ["feature_gate.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/featuregate",
    importpath = "k8s.io/component-base/featuregate",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/apimachinery/pkg/util/naming",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
