load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["sanitization.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/logs/sanitization",
    importpath = "k8s.io/component-base/logs/sanitization",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/component-base/logs/datapol:go_default_library"],
)
