load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "network",
    srcs = ["interface.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/network",
    importpath = "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/network",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/internalinterfaces",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/network/v1:network",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/network/v1alpha1",
    ],
)
