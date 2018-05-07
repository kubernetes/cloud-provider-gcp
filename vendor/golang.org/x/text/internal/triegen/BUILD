load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "compact.go",
        "print.go",
        "triegen.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/golang.org/x/text/internal/triegen",
    importpath = "golang.org/x/text/internal/triegen",
    visibility = ["//vendor/golang.org/x/text:__subpackages__"],
)
