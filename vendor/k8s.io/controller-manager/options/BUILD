load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "debugging.go",
        "generic.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/controller-manager/options",
    importpath = "k8s.io/controller-manager/options",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library",
        "//vendor/k8s.io/component-base/cli/flag:go_default_library",
        "//vendor/k8s.io/component-base/config:go_default_library",
        "//vendor/k8s.io/component-base/config/options:go_default_library",
        "//vendor/k8s.io/controller-manager/config:go_default_library",
    ],
)
