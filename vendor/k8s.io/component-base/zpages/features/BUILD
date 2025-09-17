load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "features",
    srcs = [
        "doc.go",
        "kube_features.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/zpages/features",
    importpath = "k8s.io/component-base/zpages/features",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/version",
        "//vendor/k8s.io/component-base/featuregate",
    ],
)
