load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "tokenfile",
    srcs = ["tokenfile.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authentication/token/tokenfile",
    importpath = "k8s.io/apiserver/pkg/authentication/token/tokenfile",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/authentication/authenticator",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
