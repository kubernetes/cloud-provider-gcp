load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["healthz.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/healthz",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/klog:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["healthz_test.go"],
    embed = [":go_default_library"],
)
