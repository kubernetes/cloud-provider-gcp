load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "apihelpers",
    srcs = ["helpers.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/apihelpers",
    importpath = "k8s.io/apiserver/pkg/util/apihelpers",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/api/flowcontrol/v1beta2"],
)
