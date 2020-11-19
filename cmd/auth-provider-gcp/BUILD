package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)
load("//defs:version.bzl", "version_x_defs")

go_binary(
    name = "auth-provider-gcp",
    embed = [":go_default_library"],
)

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp",
    deps = [
        "//cmd/auth-provider-gcp/app:go_default_library",
        "//vendor/github.com/spf13/cobra:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
    ],
)
