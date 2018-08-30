package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = ["chaosmonkey.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/test/e2e/chaosmonkey",
    importpath = "k8s.io/kubernetes/test/e2e/chaosmonkey",
    deps = ["@com_github_onsi_ginkgo//:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["chaosmonkey_test.go"],
    embed = [":go_default_library"],
    tags = ["e2e"],
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
