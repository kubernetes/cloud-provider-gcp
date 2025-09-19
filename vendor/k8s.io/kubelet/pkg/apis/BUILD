load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "apis",
    srcs = ["well_known_labels.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubelet/pkg/apis",
    importpath = "k8s.io/kubelet/pkg/apis",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
    ],
)
