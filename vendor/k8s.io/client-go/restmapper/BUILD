load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "restmapper",
    srcs = [
        "category_expansion.go",
        "discovery.go",
        "shortcut.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/restmapper",
    importpath = "k8s.io/client-go/restmapper",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/client-go/discovery",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
