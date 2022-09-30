load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "gcpfirewall",
    srcs = ["interface.go"],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/gcpfirewall/informers/externalversions/gcpfirewall",
    visibility = ["//visibility:public"],
    deps = [
        "//crd/client/gcpfirewall/informers/externalversions/gcpfirewall/v1beta1",
        "//crd/client/gcpfirewall/informers/externalversions/internalinterfaces",
    ],
)
