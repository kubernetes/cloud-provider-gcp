load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "endpoint",
    srcs = ["endpoint.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.etcd.io/etcd/client/v3/internal/endpoint",
    importpath = "go.etcd.io/etcd/client/v3/internal/endpoint",
    visibility = ["//vendor/go.etcd.io/etcd/client/v3:__subpackages__"],
)
