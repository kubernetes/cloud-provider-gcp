load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "util",
    srcs = ["deployment_util.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/controller/deployment/util",
    importpath = "k8s.io/kubernetes/pkg/controller/deployment/util",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/apps/v1:apps",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/equality",
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/intstr",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/client-go/kubernetes/typed/apps/v1:apps",
        "//vendor/k8s.io/client-go/listers/apps/v1:apps",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/kubernetes/pkg/controller",
        "//vendor/k8s.io/kubernetes/pkg/util/labels",
        "//vendor/k8s.io/utils/integer",
    ],
)
