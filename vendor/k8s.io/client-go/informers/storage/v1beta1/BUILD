load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v1beta1",
    srcs = [
        "csidriver.go",
        "csinode.go",
        "csistoragecapacity.go",
        "interface.go",
        "storageclass.go",
        "volumeattachment.go",
        "volumeattributesclass.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/informers/storage/v1beta1",
    importpath = "k8s.io/client-go/informers/storage/v1beta1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/storage/v1beta1",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/informers/internalinterfaces",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/listers/storage/v1beta1",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
