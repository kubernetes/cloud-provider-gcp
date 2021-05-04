load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["options.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/controller-manager/pkg/leadermigration/options",
    importpath = "k8s.io/controller-manager/pkg/leadermigration/options",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/controller-manager/config:go_default_library",
        "//vendor/k8s.io/controller-manager/pkg/leadermigration:go_default_library",
        "//vendor/k8s.io/controller-manager/pkg/leadermigration/config:go_default_library",
    ],
)
