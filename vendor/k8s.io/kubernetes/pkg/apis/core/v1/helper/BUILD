load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "helper",
    srcs = ["helpers.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/apis/core/v1/helper",
    importpath = "k8s.io/kubernetes/pkg/apis/core/v1/helper",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/resource",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apimachinery/pkg/selection",
        "//vendor/k8s.io/apimachinery/pkg/util/validation",
        "//vendor/k8s.io/kubernetes/pkg/apis/core/helper",
    ],
)
