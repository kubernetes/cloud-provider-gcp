load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "bootstrap",
    srcs = ["default.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/apis/flowcontrol/bootstrap",
    importpath = "k8s.io/apiserver/pkg/apis/flowcontrol/bootstrap",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/coordination/v1:coordination",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/api/flowcontrol/v1:flowcontrol",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apiserver/pkg/authentication/serviceaccount",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/utils/ptr",
    ],
)
