load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cache",
    srcs = [
        "expiring.go",
        "lruexpirecache.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/util/cache",
    importpath = "k8s.io/apimachinery/pkg/util/cache",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/utils/clock"],
)
