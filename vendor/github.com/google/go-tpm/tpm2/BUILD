load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "constants.go",
        "error.go",
        "kdf.go",
        "structures.go",
        "tpm2.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/go-tpm/tpm2",
    importpath = "github.com/google/go-tpm/tpm2",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/google/go-tpm/tpmutil:go_default_library"],
)
