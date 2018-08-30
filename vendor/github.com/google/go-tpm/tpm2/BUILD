load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "constants.go",
        "structures.go",
        "tpm2.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/google/go-tpm/tpm2",
    importpath = "github.com/google/go-tpm/tpm2",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/google/go-tpm/tpmutil:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "encoding_test.go",
        "tpm2_linux_test.go",
        "tpm2_test.go",
        "tpm2_windows_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/google/go-tpm/tpmutil:go_default_library",
    ],
)
