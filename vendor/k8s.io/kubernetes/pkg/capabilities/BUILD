load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "capabilities",
    srcs = [
        "capabilities.go",
        "doc.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/capabilities",
    importpath = "k8s.io/kubernetes/pkg/capabilities",
    visibility = ["//visibility:public"],
)
