load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "pods",
    srcs = ["helpers.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/apis/core/pods",
    importpath = "k8s.io/kubernetes/pkg/apis/core/pods",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field",
        "//vendor/k8s.io/kubernetes/pkg/apis/core",
        "//vendor/k8s.io/kubernetes/pkg/fieldpath",
    ],
)
