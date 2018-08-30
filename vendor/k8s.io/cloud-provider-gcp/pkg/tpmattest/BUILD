load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["tpmattest.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/cloud-provider-gcp/pkg/tpmattest",
    importpath = "k8s.io/cloud-provider-gcp/pkg/tpmattest",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/google/go-tpm/tpm2:go_default_library"],
)
