load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "events",
    srcs = ["interfaces.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/tools/internal/events",
    importpath = "k8s.io/client-go/tools/internal/events",
    visibility = ["//vendor/k8s.io/client-go/tools:__subpackages__"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
