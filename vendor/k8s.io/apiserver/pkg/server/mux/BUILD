load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "mux",
    srcs = [
        "doc.go",
        "pathrecorder.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/server/mux",
    importpath = "k8s.io/apiserver/pkg/server/mux",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
