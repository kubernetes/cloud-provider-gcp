load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "storage",
    srcs = [
        "doc.go",
        "resource_config.go",
        "resource_encoding_config.go",
        "storage_codec.go",
        "storage_factory.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/server/storage",
    importpath = "k8s.io/apiserver/pkg/server/storage",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer/recognizer",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apimachinery/pkg/util/version",
        "//vendor/k8s.io/apiserver/pkg/storage/storagebackend",
        "//vendor/k8s.io/apiserver/pkg/util/compatibility",
        "//vendor/k8s.io/component-base/compatibility",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
