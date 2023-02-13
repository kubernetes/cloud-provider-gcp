load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "queueset",
    srcs = [
        "doc.go",
        "fifo_list.go",
        "queueset.go",
        "types.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/flowcontrol/fairqueuing/queueset",
    importpath = "k8s.io/apiserver/pkg/util/flowcontrol/fairqueuing/queueset",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/endpoints/request",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/debug",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/fairqueuing",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/fairqueuing/eventclock",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/fairqueuing/promise",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/metrics",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/request",
        "//vendor/k8s.io/apiserver/pkg/util/shufflesharding",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/clock/testing",
    ],
)
