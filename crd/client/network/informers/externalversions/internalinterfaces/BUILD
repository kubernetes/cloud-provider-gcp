load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internalinterfaces",
    srcs = ["factory_interfaces.go"],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/internalinterfaces",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned",
    ],
)
