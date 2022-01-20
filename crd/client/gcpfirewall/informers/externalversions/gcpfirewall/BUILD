load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["interface.go"],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/gcpfirewall/informers/externalversions/gcpfirewall",
    visibility = ["//visibility:public"],
    deps = [
        "//crd/client/gcpfirewall/informers/externalversions/gcpfirewall/v1alpha1:go_default_library",
        "//crd/client/gcpfirewall/informers/externalversions/internalinterfaces:go_default_library",
    ],
)
