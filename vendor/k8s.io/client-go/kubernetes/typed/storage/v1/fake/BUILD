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
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/kubernetes/typed/storage/v1/fake",
    importpath = "k8s.io/client-go/kubernetes/typed/storage/v1/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/storage/v1:storage",
        "//vendor/k8s.io/client-go/applyconfigurations/storage/v1:storage",
        "//vendor/k8s.io/client-go/gentype",
        "//vendor/k8s.io/client-go/kubernetes/typed/storage/v1:storage",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/testing",
    ],
)
