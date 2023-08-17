load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "network",
    srcs = ["interface.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network",
    importpath = "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/internalinterfaces",
        "//vendor/k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network/v1:network",
        "//vendor/k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network/v1alpha1",
    ],
)
