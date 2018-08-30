load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["certificates.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apiserver/pkg/storage/etcd/testing/testingcert",
    importpath = "k8s.io/apiserver/pkg/storage/etcd/testing/testingcert",
    visibility = ["//visibility:public"],
)
