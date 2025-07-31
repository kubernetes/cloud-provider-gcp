load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "deprecation",
    srcs = ["deprecation.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/deprecation",
    importpath = "k8s.io/apiserver/pkg/endpoints/deprecation",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/version",
    ],
)
