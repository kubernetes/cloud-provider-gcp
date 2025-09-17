load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metrics",
    srcs = ["metrics.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/server/options/encryptionconfig/metrics",
    importpath = "k8s.io/apiserver/pkg/server/options/encryptionconfig/metrics",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/util/configmetrics",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
    ],
)
