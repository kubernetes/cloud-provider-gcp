load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "responsewriter",
    srcs = [
        "fake.go",
        "wrapper.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/responsewriter",
    importpath = "k8s.io/apiserver/pkg/endpoints/responsewriter",
    visibility = ["//visibility:public"],
)
