load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "parse.go",
        "xfs.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/prometheus/procfs/xfs",
    importpath = "github.com/prometheus/procfs/xfs",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/prometheus/procfs/internal/util:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["parse_test.go"],
    embed = [":go_default_library"],
    deps = ["//vendor/github.com/prometheus/procfs:go_default_library"],
)
