load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["helpers.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-helpers/node/topology",
    importpath = "k8s.io/component-helpers/node/topology",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/api/core/v1:go_default_library"],
)
