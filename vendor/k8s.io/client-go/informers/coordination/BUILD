load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "coordination",
    srcs = ["interface.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/informers/coordination",
    importpath = "k8s.io/client-go/informers/coordination",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/client-go/informers/coordination/v1:coordination",
        "//vendor/k8s.io/client-go/informers/coordination/v1alpha2",
        "//vendor/k8s.io/client-go/informers/coordination/v1beta1",
        "//vendor/k8s.io/client-go/informers/internalinterfaces",
    ],
)
