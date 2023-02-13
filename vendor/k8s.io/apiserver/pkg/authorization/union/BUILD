load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "union",
    srcs = ["union.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authorization/union",
    importpath = "k8s.io/apiserver/pkg/authorization/union",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/errors",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/apiserver/pkg/authorization/authorizer",
    ],
)
