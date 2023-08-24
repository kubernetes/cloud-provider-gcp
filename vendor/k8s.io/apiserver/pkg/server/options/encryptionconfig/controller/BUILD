load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "controller",
    srcs = ["controller.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/server/options/encryptionconfig/controller",
    importpath = "k8s.io/apiserver/pkg/server/options/encryptionconfig/controller",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/fsnotify/fsnotify",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/server/healthz",
        "//vendor/k8s.io/apiserver/pkg/server/options/encryptionconfig",
        "//vendor/k8s.io/apiserver/pkg/server/options/encryptionconfig/metrics",
        "//vendor/k8s.io/client-go/util/workqueue",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
