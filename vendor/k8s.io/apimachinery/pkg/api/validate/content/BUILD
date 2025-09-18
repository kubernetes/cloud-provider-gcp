load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "content",
    srcs = ["errors.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/api/validate/content",
    importpath = "k8s.io/apimachinery/pkg/api/validate/content",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/apimachinery/pkg/api/validate/constraints"],
)
