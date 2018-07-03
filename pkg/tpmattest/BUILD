load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["tpmattest.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/tpmattest",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/google/go-tpm/tpm2:go_default_library"],
)
