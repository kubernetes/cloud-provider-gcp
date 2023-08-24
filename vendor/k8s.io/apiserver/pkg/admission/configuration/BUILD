load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "configuration",
    srcs = [
        "configuration_manager.go",
        "mutating_webhook_manager.go",
        "validating_webhook_manager.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/admission/configuration",
    importpath = "k8s.io/apiserver/pkg/admission/configuration",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/admissionregistration/v1:admissionregistration",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/generic",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/listers/admissionregistration/v1:admissionregistration",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/tools/cache/synctrack",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
