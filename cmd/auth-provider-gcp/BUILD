package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)
load("//defs:version.bzl", "version_x_defs")

go_binary(
    name = "auth-provider-gcp",
    embed = [":auth-provider-gcp_lib"],
)

go_library(
    name = "auth-provider-gcp_lib",
    srcs = ["main.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp",
    deps = [
        "//cmd/auth-provider-gcp/app",
        "//vendor/github.com/spf13/cobra",
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
