load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "network",
    srcs = [
        "gkenetworkparams.go",
        "interface.go",
        "network.go",
        "networkinterface.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//crd/apis/network/v1",
        "//crd/client/network/clientset/versioned",
        "//crd/client/network/informers/externalversions/internalinterfaces",
        "//crd/client/network/listers/network/v1:network",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
