load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "resource",
    srcs = ["interface.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/informers/resource",
    importpath = "k8s.io/client-go/informers/resource",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/client-go/informers/internalinterfaces",
        "//vendor/k8s.io/client-go/informers/resource/v1alpha3",
    ],
)
