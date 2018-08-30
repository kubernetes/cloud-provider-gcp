load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["metrics.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apiserver/pkg/storage/etcd/metrics",
    importpath = "k8s.io/apiserver/pkg/storage/etcd/metrics",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/prometheus/client_golang/prometheus:go_default_library"],
)
