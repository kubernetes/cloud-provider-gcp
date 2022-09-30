load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metadatainformer",
    srcs = [
        "informer.go",
        "interface.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/metadata/metadatainformer",
    importpath = "k8s.io/client-go/metadata/metadatainformer",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/metadata",
        "//vendor/k8s.io/client-go/metadata/metadatalister",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
