load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "bad-no-name.go",
        "bad-no-open.go",
        "bad-unclosed.go",
        "snips.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/internal/readme/testdata",
    importpath = "cloud.google.com/go/internal/readme/testdata",
    visibility = ["//vendor/cloud.google.com/go:__subpackages__"],
)
