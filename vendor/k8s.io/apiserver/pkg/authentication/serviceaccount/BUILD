load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "serviceaccount",
    srcs = ["util.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authentication/serviceaccount",
    importpath = "k8s.io/apiserver/pkg/authentication/serviceaccount",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/api/validation",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/client-go/kubernetes/typed/core/v1:core",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
