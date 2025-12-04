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

go_binary(
    name = "auth-provider-gcp_linux_amd64",
    embed = [":auth-provider-gcp_lib"],
    goarch = "amd64",
    goos = "linux",
)

go_binary(
    name = "auth-provider-gcp_linux_arm64",
    embed = [":auth-provider-gcp_lib"],
    goarch = "arm64",
    goos = "linux",
)

go_binary(
    name = "auth-provider-gcp_windows_amd64",
    embed = [":auth-provider-gcp_lib"],
    goarch = "amd64",
    goos = "windows",
)

genrule(
    name = "release",
    srcs = [
        ":auth-provider-gcp_linux_amd64",
        ":auth-provider-gcp_linux_arm64",
        ":auth-provider-gcp_windows_amd64",
    ],
    outs = [
        "output/linux/amd64/auth-provider-gcp",
        "output/linux/arm64/auth-provider-gcp",
        "output/windows/amd64/auth-provider-gcp.exe",
    ],
    cmd = """
        cp $(location :auth-provider-gcp_linux_amd64) $(location output/linux/amd64/auth-provider-gcp)
        cp $(location :auth-provider-gcp_linux_arm64) $(location output/linux/arm64/auth-provider-gcp)
        cp $(location :auth-provider-gcp_windows_amd64) $(location output/windows/amd64/auth-provider-gcp.exe)
    """,
)

go_library(
    name = "auth-provider-gcp_lib",
    srcs = ["main.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp",
    deps = [
        "//cmd/auth-provider-gcp/app",
        "//providers/gce/gcpcredential",
        "//vendor/github.com/spf13/cobra",
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
