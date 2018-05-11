load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "uritemplates.go",
        "utils.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/google.golang.org/api/googleapi/internal/uritemplates",
    importpath = "google.golang.org/api/googleapi/internal/uritemplates",
    visibility = ["//vendor/google.golang.org/api/googleapi:__subpackages__"],
)
