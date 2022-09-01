load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "errors",
    srcs = [
        "doc.go",
        "storage.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/errors",
    importpath = "k8s.io/apiserver/pkg/storage/errors",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apiserver/pkg/storage",
    ],
)
