load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "configmetrics",
    srcs = ["info_collector.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/configmetrics",
    importpath = "k8s.io/apiserver/pkg/util/configmetrics",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/utils/ptr",
    ],
)
