load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["tpmkey.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/awly/tpmtls/tpmkey",
    importpath = "github.com/awly/tpmtls/tpmkey",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/go-tpm/tpm2:go_default_library",
        "//vendor/github.com/google/go-tpm/tpmutil:go_default_library",
    ],
)
