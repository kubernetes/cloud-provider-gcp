load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "storageversion",
    srcs = [
        "manager.go",
        "updater.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storageversion",
    importpath = "k8s.io/apiserver/pkg/storageversion",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/apiserverinternal/v1alpha1",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/component-base/metrics/prometheus/workqueue",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
