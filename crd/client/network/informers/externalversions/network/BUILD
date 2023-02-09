load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "network",
    srcs = ["interface.go"],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network",
    visibility = ["//visibility:public"],
    deps = [
        "//crd/client/network/informers/externalversions/internalinterfaces",
        "//crd/client/network/informers/externalversions/network/v1:network",
        "//crd/client/network/informers/externalversions/network/v1alpha1",
    ],
)
