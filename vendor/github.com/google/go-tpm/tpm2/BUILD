load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "constants.go",
        "kdf.go",
        "structures.go",
        "tpm2.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/google/go-tpm/tpm2",
    importpath = "github.com/google/go-tpm/tpm2",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/google/go-tpm/tpmutil:go_default_library"],
)
