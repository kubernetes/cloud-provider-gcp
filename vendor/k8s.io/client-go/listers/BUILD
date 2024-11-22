load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "listers",
    srcs = [
        "doc.go",
        "generic_helpers.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/listers",
    importpath = "k8s.io/client-go/listers",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
