load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fieldpath",
    srcs = [
        "doc.go",
        "fieldpath.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/fieldpath",
    importpath = "k8s.io/kubernetes/pkg/fieldpath",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/apimachinery/pkg/util/validation",
    ],
)
