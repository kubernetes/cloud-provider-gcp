load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "capabilities.go",
        "doc.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/capabilities",
    importpath = "k8s.io/kubernetes/pkg/capabilities",
    visibility = ["//visibility:public"],
)
