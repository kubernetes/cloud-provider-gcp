load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v1alpha1",
    srcs = [
        "gkenetworkparamset.go",
        "interface.go",
        "network.go",
        "networkinterface.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network/v1alpha1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/cloud-provider-gcp/crd/apis/network/v1alpha1",
        "//vendor/k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned",
        "//vendor/k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/internalinterfaces",
        "//vendor/k8s.io/cloud-provider-gcp/crd/client/network/listers/network/v1alpha1",
    ],
)
