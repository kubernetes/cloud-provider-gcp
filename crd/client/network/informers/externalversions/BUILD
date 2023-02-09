load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "externalversions",
    srcs = [
        "factory.go",
        "generic.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/client-go/tools/cache",
        "//crd/apis/network/v1:network",
        "//crd/apis/network/v1alpha1",
        "//crd/client/network/clientset/versioned",
        "//crd/client/network/informers/externalversions/internalinterfaces",
        "//crd/client/network/informers/externalversions/network",
    ],
)
