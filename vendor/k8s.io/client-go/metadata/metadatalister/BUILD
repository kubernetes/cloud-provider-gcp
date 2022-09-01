load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metadatalister",
    srcs = [
        "interface.go",
        "lister.go",
        "shim.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/metadata/metadatalister",
    importpath = "k8s.io/client-go/metadata/metadatalister",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
