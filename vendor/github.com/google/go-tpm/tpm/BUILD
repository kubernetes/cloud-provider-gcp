load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "commands.go",
        "constants.go",
        "errors.go",
        "pcrs.go",
        "structures.go",
        "tpm.go",
        "verify.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/google/go-tpm/tpm",
    importpath = "github.com/google/go-tpm/tpm",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/google/go-tpm/tpmutil:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "pcrs_test.go",
        "tpm_linux_test.go",
        "tpm_test.go",
        "tpm_windows_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/google/go-tpm/tpmutil:go_default_library",
    ],
)
