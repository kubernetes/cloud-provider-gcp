load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "options",
    srcs = [
        "debugging.go",
        "generic.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/controller-manager/options",
    importpath = "k8s.io/controller-manager/options",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/component-base/cli/flag",
        "//vendor/k8s.io/component-base/config",
        "//vendor/k8s.io/component-base/config/options",
        "//vendor/k8s.io/controller-manager/config",
        "//vendor/k8s.io/controller-manager/pkg/leadermigration/options",
    ],
)
