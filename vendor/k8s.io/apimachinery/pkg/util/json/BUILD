load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "json",
    srcs = ["json.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/util/json",
    importpath = "k8s.io/apimachinery/pkg/util/json",
    visibility = ["//visibility:public"],
    deps = ["//vendor/sigs.k8s.io/json"],
)
