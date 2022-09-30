load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "validating",
    srcs = [
        "dispatcher.go",
        "doc.go",
        "plugin.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/validating",
    importpath = "k8s.io/apiserver/pkg/admission/plugin/webhook/validating",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/admissionregistration/v1:admissionregistration",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apiserver/pkg/admission",
        "//vendor/k8s.io/apiserver/pkg/admission/configuration",
        "//vendor/k8s.io/apiserver/pkg/admission/metrics",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/errors",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/generic",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/request",
        "//vendor/k8s.io/apiserver/pkg/endpoints/request",
        "//vendor/k8s.io/apiserver/pkg/util/webhook",
        "//vendor/k8s.io/apiserver/pkg/warning",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/trace",
    ],
)
