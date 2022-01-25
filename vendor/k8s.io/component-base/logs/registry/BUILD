load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["registry.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/logs/registry",
    importpath = "k8s.io/component-base/logs/registry",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/go-logr/logr:go_default_library",
        "//vendor/k8s.io/component-base/config:go_default_library",
    ],
)
