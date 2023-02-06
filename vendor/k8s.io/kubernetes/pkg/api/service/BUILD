load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "service",
    srcs = ["util.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/api/service",
    importpath = "k8s.io/kubernetes/pkg/api/service",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/apis/core",
        "//vendor/k8s.io/utils/net",
    ],
)
