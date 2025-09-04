load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "operation",
    srcs = ["operation.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/api/operation",
    importpath = "k8s.io/apimachinery/pkg/api/operation",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/apimachinery/pkg/util/sets"],
)
