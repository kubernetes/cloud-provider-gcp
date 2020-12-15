load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["legacy.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/discovery/cached",
    importpath = "k8s.io/client-go/discovery/cached",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/client-go/discovery:go_default_library",
        "//vendor/k8s.io/client-go/discovery/cached/memory:go_default_library",
    ],
)
