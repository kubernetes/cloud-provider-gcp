load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["uuid.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apimachinery/pkg/util/uuid",
    importpath = "k8s.io/apimachinery/pkg/util/uuid",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "@com_github_pborman_uuid//:go_default_library",
    ],
)
