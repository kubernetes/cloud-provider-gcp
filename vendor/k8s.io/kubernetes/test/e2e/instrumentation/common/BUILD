package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = ["framework.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/test/e2e/instrumentation/common",
    importpath = "k8s.io/kubernetes/test/e2e/instrumentation/common",
    deps = ["@com_github_onsi_ginkgo//:go_default_library"],
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
)
