load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "pod",
    srcs = ["util.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/api/v1/pod",
    importpath = "k8s.io/kubernetes/pkg/api/v1/pod",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/util/intstr",
    ],
)
