load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/google/go-tpm/examples/tpm2-seal-unseal",
    importpath = "github.com/google/go-tpm/examples/tpm2-seal-unseal",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/github.com/google/go-tpm/tpm2:go_default_library",
        "//vendor/github.com/google/go-tpm/tpmutil:go_default_library",
    ],
)

go_binary(
    name = "tpm2-seal-unseal",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
