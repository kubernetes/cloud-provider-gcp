load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "resource",
    srcs = ["helpers.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-helpers/resource",
    importpath = "k8s.io/component-helpers/resource",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
    ],
)
