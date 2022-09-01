load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "healthz",
    srcs = ["healthz.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/healthz",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/klog/v2:klog"],
)

go_test(
    name = "healthz_test",
    srcs = ["healthz_test.go"],
    embed = [":healthz"],
)
