load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "labels",
    srcs = [
        "doc.go",
        "labels.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/util/labels",
    importpath = "k8s.io/kubernetes/pkg/util/labels",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta"],
)
