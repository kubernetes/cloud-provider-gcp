load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "networking",
    srcs = ["interface.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/informers/networking",
    importpath = "k8s.io/client-go/informers/networking",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/client-go/informers/internalinterfaces",
        "//vendor/k8s.io/client-go/informers/networking/v1:networking",
        "//vendor/k8s.io/client-go/informers/networking/v1beta1",
    ],
)
