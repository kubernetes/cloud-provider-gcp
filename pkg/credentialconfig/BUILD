package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "credentialconfig",
    srcs = [
        "config.go",
        "provider.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/pkg/credentialconfig",
    deps = [
        "//vendor/k8s.io/klog/v2:klog",
    ],
)

go_test(
    name = "credentialconfig_test",
    srcs = [
        "config_test.go",
        "provider_test.go",
    ],
    embed = [":credentialconfig"],
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
