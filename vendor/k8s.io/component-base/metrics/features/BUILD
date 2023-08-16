load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "features",
    srcs = ["kube_features.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/metrics/features",
    importpath = "k8s.io/component-base/metrics/features",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/component-base/featuregate"],
)
