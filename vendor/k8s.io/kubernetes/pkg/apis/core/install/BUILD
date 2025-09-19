load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "install",
    srcs = ["install.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/apis/core/install",
    importpath = "k8s.io/kubernetes/pkg/apis/core/install",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/kubernetes/pkg/api/legacyscheme",
        "//vendor/k8s.io/kubernetes/pkg/apis/core",
        "//vendor/k8s.io/kubernetes/pkg/apis/core/v1:core",
    ],
)
