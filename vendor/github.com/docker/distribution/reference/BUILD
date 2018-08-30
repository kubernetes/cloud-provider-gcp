load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "helpers.go",
        "normalize.go",
        "reference.go",
        "regexp.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/reference",
    importpath = "github.com/docker/distribution/reference",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/docker/distribution/digestset:go_default_library",
        "//vendor/github.com/opencontainers/go-digest:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "normalize_test.go",
        "reference_test.go",
        "regexp_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/docker/distribution/digestset:go_default_library",
        "//vendor/github.com/opencontainers/go-digest:go_default_library",
    ],
)
