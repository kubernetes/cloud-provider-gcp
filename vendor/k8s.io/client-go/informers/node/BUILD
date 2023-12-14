load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "node",
    srcs = ["interface.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/informers/node",
    importpath = "k8s.io/client-go/informers/node",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/client-go/informers/internalinterfaces",
        "//vendor/k8s.io/client-go/informers/node/v1:node",
        "//vendor/k8s.io/client-go/informers/node/v1alpha1",
        "//vendor/k8s.io/client-go/informers/node/v1beta1",
    ],
)
