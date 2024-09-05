load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "stdlib",
    srcs = [
        "manifest.go",
        "stdlib.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/internal/stdlib",
    importpath = "golang.org/x/tools/internal/stdlib",
    visibility = ["//vendor/golang.org/x/tools:__subpackages__"],
)
