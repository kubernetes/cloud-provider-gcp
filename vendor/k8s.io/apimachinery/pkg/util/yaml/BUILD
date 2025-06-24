load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "yaml",
    srcs = ["decoder.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/util/yaml",
    importpath = "k8s.io/apimachinery/pkg/util/yaml",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/json",
        "//vendor/sigs.k8s.io/yaml",
    ],
)
