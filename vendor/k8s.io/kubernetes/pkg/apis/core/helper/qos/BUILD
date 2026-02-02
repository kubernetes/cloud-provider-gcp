load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "qos",
    srcs = ["qos.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/apis/core/helper/qos",
    importpath = "k8s.io/kubernetes/pkg/apis/core/helper/qos",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/resource",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/kubernetes/pkg/apis/core",
        "//vendor/k8s.io/kubernetes/pkg/features",
    ],
)
