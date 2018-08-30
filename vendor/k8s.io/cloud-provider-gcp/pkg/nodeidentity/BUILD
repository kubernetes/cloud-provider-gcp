load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["id.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/cloud-provider-gcp/pkg/nodeidentity",
    importpath = "k8s.io/cloud-provider-gcp/pkg/nodeidentity",
    visibility = ["//visibility:public"],
    deps = ["//vendor/cloud.google.com/go/compute/metadata:go_default_library"],
)
