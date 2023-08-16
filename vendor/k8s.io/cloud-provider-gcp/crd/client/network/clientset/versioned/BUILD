load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "versioned",
    srcs = [
        "clientset.go",
        "doc.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned",
    importpath = "k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/client-go/discovery",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/util/flowcontrol",
        "//vendor/k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned/typed/network/v1:network",
        "//vendor/k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned/typed/network/v1alpha1",
    ],
)
