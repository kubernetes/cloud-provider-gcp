load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "parsers",
    srcs = ["parsers.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/util/parsers",
    importpath = "k8s.io/kubernetes/pkg/util/parsers",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/docker/distribution/reference"],
)
