load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "sysctl",
    srcs = [
        "namespace.go",
        "sysctl.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-helpers/node/util/sysctl",
    importpath = "k8s.io/component-helpers/node/util/sysctl",
    visibility = ["//visibility:public"],
)
