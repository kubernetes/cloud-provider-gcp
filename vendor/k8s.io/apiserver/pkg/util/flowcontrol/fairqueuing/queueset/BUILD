load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
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
        "//vendor/k8s.io/apiserver/pkg/endpoints/request:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/debug:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/fairqueuing:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/fairqueuing/eventclock:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/fairqueuing/promise:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/metrics:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/request:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/shufflesharding:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
        "//vendor/k8s.io/utils/clock/testing:go_default_library",
    ],
)
