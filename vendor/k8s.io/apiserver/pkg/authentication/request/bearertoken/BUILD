load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "bearertoken",
    srcs = ["bearertoken.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authentication/request/bearertoken",
    importpath = "k8s.io/apiserver/pkg/authentication/request/bearertoken",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/authentication/authenticator",
        "//vendor/k8s.io/apiserver/pkg/warning",
    ],
)
