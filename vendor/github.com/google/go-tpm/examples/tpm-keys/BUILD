load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["tpm-keys.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/google/go-tpm/examples/tpm-keys",
    importpath = "github.com/google/go-tpm/examples/tpm-keys",
    visibility = ["//visibility:private"],
    deps = ["//vendor/github.com/google/go-tpm/tpm:go_default_library"],
)

go_binary(
    name = "tpm-keys",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
