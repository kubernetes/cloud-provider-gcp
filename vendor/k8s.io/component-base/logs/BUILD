load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "config.go",
        "logs.go",
        "options.go",
        "validate.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/logs",
    importpath = "k8s.io/component-base/logs",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/errors:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/component-base/cli/flag:go_default_library",
        "//vendor/k8s.io/component-base/config:go_default_library",
        "//vendor/k8s.io/component-base/config/v1alpha1:go_default_library",
        "//vendor/k8s.io/component-base/logs/registry:go_default_library",
        "//vendor/k8s.io/component-base/logs/sanitization:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
    ],
)
