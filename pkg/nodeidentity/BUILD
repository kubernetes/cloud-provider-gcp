load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "nodeidentity",
    srcs = ["id.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/nodeidentity",
    visibility = ["//visibility:public"],
    deps = ["//vendor/cloud.google.com/go/compute/metadata"],
)
