load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "install",
    srcs = ["install.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/cloud-provider/config/install",
    importpath = "k8s.io/cloud-provider/config/install",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/cloud-provider/config",
        "//vendor/k8s.io/cloud-provider/config/v1alpha1",
    ],
)
