load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "remotecommand",
    srcs = ["constants.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/util/remotecommand",
    importpath = "k8s.io/apimachinery/pkg/util/remotecommand",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta"],
)
