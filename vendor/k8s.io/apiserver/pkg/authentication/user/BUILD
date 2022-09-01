load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "user",
    srcs = [
        "doc.go",
        "user.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authentication/user",
    importpath = "k8s.io/apiserver/pkg/authentication/user",
    visibility = ["//visibility:public"],
)
