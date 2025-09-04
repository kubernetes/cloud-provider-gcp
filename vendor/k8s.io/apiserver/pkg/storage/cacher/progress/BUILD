load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "progress",
    srcs = ["watch_progress.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/cacher/progress",
    importpath = "k8s.io/apiserver/pkg/storage/cacher/progress",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc/metadata",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/clock",
    ],
)
