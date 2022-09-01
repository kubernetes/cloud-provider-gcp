load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "schemaconv",
    srcs = ["smd.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kube-openapi/pkg/schemaconv",
    importpath = "k8s.io/kube-openapi/pkg/schemaconv",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/kube-openapi/pkg/util/proto",
        "//vendor/sigs.k8s.io/structured-merge-diff/v4/schema",
    ],
)
