load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["fake.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/proxy/util/testing",
    importpath = "k8s.io/kubernetes/pkg/proxy/util/testing",
    visibility = ["//visibility:public"],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
