load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "authenticator",
    srcs = [
        "audagnostic.go",
        "audiences.go",
        "interfaces.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authentication/authenticator",
    importpath = "k8s.io/apiserver/pkg/authentication/authenticator",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/apiserver/pkg/authentication/user"],
)
