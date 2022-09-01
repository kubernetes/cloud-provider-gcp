load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "object",
    srcs = [
        "doc.go",
        "matcher.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/object",
    importpath = "k8s.io/apiserver/pkg/admission/plugin/webhook/object",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apiserver/pkg/admission",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
