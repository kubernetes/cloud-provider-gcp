load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "common.go",
        "extend_pcr.go",
        "generate.go",
        "sign.go",
        "tpm-sign.go",
        "verify.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/google/go-tpm/examples/tpm-sign",
    importpath = "github.com/google/go-tpm/examples/tpm-sign",
    visibility = ["//visibility:private"],
    deps = ["//vendor/github.com/google/go-tpm/tpm:go_default_library"],
)

go_binary(
    name = "tpm-sign",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
