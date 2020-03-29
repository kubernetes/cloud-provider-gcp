load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["providers.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager/app/cloudprovider",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/legacy-cloud-providers/gce:go_default_library"],
)
