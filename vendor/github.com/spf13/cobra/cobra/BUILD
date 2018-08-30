load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/spf13/cobra/cobra",
    importpath = "github.com/spf13/cobra/cobra",
    visibility = ["//visibility:private"],
    deps = ["//vendor/github.com/spf13/cobra/cobra/cmd:go_default_library"],
)

go_binary(
    name = "cobra",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
