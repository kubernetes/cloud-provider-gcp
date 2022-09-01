load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "generic",
    srcs = [
        "doc.go",
        "matcher.go",
        "options.go",
        "storage_decorator.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/registry/generic",
    importpath = "k8s.io/apiserver/pkg/registry/generic",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/fields",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apiserver/pkg/storage",
        "//vendor/k8s.io/apiserver/pkg/storage/storagebackend",
        "//vendor/k8s.io/apiserver/pkg/storage/storagebackend/factory",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/request",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
