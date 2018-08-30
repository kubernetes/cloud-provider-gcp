load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "pclntab.go",
        "symtab.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/gosym",
    importpath = "cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/gosym",
    visibility = ["//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent:__subpackages__"],
)

go_test(
    name = "go_default_test",
    srcs = ["pclntab_test.go"],
    embed = [":go_default_library"],
)
