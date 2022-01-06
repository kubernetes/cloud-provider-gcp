load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "metrics.go",
        "observer.go",
        "sample_and_watermark.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/flowcontrol/metrics",
    importpath = "k8s.io/apiserver/pkg/util/flowcontrol/metrics",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/component-base/metrics:go_default_library",
        "//vendor/k8s.io/component-base/metrics/legacyregistry:go_default_library",
        "//vendor/k8s.io/component-base/metrics/testutil:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
        "//vendor/k8s.io/utils/clock:go_default_library",
    ],
)
