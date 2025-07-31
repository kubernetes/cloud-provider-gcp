load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "errors",
    srcs = [
        "api.go",
        "doc.go",
        "headers.go",
        "schema.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kube-openapi/pkg/validation/errors",
    importpath = "k8s.io/kube-openapi/pkg/validation/errors",
    visibility = ["//visibility:public"],
)
