load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "schemamutation",
    srcs = ["walker.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kube-openapi/pkg/schemamutation",
    importpath = "k8s.io/kube-openapi/pkg/schemamutation",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/kube-openapi/pkg/validation/spec"],
)
