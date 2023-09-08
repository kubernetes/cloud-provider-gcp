load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "taints",
    srcs = ["taints.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/util/taints",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/api/core/v1:core"],
)

go_test(
    name = "taints_test",
    srcs = ["taints_test.go"],
    embed = [":taints"],
    deps = ["//vendor/k8s.io/api/core/v1:core"],
)
