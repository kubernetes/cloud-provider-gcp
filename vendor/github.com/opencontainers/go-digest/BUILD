load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "algorithm.go",
        "digest.go",
        "digester.go",
        "doc.go",
        "verifiers.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/opencontainers/go-digest",
    importpath = "github.com/opencontainers/go-digest",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "algorithm_test.go",
        "digest_test.go",
        "verifiers_test.go",
    ],
    embed = [":go_default_library"],
)
