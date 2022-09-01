load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metrics",
    srcs = [
        "interface.go",
        "metrics.go",
        "timing_ratio_histogram.go",
        "vec_element_pair.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/flowcontrol/metrics",
    importpath = "k8s.io/apiserver/pkg/util/flowcontrol/metrics",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/endpoints/metrics",
        "//vendor/k8s.io/apiserver/pkg/endpoints/request",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
        "//vendor/k8s.io/component-base/metrics/testutil",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
