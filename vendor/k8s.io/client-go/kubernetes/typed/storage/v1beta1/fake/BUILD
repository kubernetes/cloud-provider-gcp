load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fake",
    srcs = [
        "doc.go",
        "fake_csidriver.go",
        "fake_csinode.go",
        "fake_csistoragecapacity.go",
        "fake_storage_client.go",
        "fake_storageclass.go",
        "fake_volumeattachment.go",
        "fake_volumeattributesclass.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/kubernetes/typed/storage/v1beta1/fake",
    importpath = "k8s.io/client-go/kubernetes/typed/storage/v1beta1/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/storage/v1beta1",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/applyconfigurations/storage/v1beta1",
        "//vendor/k8s.io/client-go/kubernetes/typed/storage/v1beta1",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/testing",
    ],
)
