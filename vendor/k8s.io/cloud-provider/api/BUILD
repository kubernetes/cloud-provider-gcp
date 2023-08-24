load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "api",
    srcs = [
        "retry_error.go",
        "well_known_annotations.go",
        "well_known_taints.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/cloud-provider/api",
    importpath = "k8s.io/cloud-provider/api",
    visibility = ["//visibility:public"],
)
