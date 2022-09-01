load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cached",
    srcs = ["legacy.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/discovery/cached",
    importpath = "k8s.io/client-go/discovery/cached",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/client-go/discovery",
        "//vendor/k8s.io/client-go/discovery/cached/memory",
    ],
)
