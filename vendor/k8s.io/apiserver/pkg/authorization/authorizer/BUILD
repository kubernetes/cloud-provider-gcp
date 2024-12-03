load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "authorizer",
    srcs = [
        "interfaces.go",
        "rule.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authorization/authorizer",
    importpath = "k8s.io/apiserver/pkg/authorization/authorizer",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/fields",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
    ],
)
