load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "request",
    srcs = [
        "config.go",
        "list_work_estimator.go",
        "mutating_work_estimator.go",
        "object_count_tracker.go",
        "seat_seconds.go",
        "width.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/flowcontrol/request",
    importpath = "k8s.io/apiserver/pkg/util/flowcontrol/request",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/endpoints/request",
        "//vendor/k8s.io/apiserver/pkg/features",
        "//vendor/k8s.io/apiserver/pkg/storage",
        "//vendor/k8s.io/apiserver/pkg/storage/feature",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/metrics",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/clock",
    ],
)
