load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["lru.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/utils/lru",
    importpath = "k8s.io/utils/lru",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/utils/internal/third_party/forked/golang/golang-lru:go_default_library"],
)
