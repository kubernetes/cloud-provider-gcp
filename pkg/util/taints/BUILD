load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["taints.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/util/taints",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/api/core/v1:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["taints_test.go"],
    embed = [":go_default_library"],
    deps = ["//vendor/k8s.io/api/core/v1:go_default_library"],
)
