load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fieldmanager",
    srcs = [
        "admission.go",
        "equality.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager",
    importpath = "k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/equality",
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/conversion",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/managedfields",
        "//vendor/k8s.io/apiserver/pkg/admission",
        "//vendor/k8s.io/apiserver/pkg/endpoints/metrics",
        "//vendor/k8s.io/apiserver/pkg/warning",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
