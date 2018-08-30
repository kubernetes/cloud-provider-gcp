load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["info.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/prometheus/common/version",
    importpath = "github.com/prometheus/common/version",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/prometheus/client_golang/prometheus:go_default_library"],
)
