load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "testinit",
    srcs = ["testinit.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/logs/testinit",
    importpath = "k8s.io/component-base/logs/testinit",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/component-base/logs"],
)
