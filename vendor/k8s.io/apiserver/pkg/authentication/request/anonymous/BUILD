load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "anonymous",
    srcs = ["anonymous.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authentication/request/anonymous",
    importpath = "k8s.io/apiserver/pkg/authentication/request/anonymous",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/apis/apiserver",
        "//vendor/k8s.io/apiserver/pkg/authentication/authenticator",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
    ],
)
