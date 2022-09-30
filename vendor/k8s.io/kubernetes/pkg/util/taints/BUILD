load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "taints",
    srcs = ["taints.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/util/taints",
    importpath = "k8s.io/kubernetes/pkg/util/taints",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/util/errors",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apimachinery/pkg/util/validation",
        "//vendor/k8s.io/kubernetes/pkg/apis/core/helper",
    ],
)
