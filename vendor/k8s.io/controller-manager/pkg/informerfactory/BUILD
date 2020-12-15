load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["informer_factory.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/controller-manager/pkg/informerfactory",
    importpath = "k8s.io/controller-manager/pkg/informerfactory",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/client-go/informers:go_default_library",
        "//vendor/k8s.io/client-go/metadata/metadatainformer:go_default_library",
    ],
)
