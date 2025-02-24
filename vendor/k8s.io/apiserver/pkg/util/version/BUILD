load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "version",
    srcs = [
        "registry.go",
        "version.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/version",
    importpath = "k8s.io/apiserver/pkg/util/version",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/version",
        "//vendor/k8s.io/component-base/cli/flag",
        "//vendor/k8s.io/component-base/featuregate",
        "//vendor/k8s.io/component-base/version",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
