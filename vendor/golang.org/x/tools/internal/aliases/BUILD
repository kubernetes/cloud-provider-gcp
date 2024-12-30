load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "aliases",
    srcs = [
        "aliases.go",
        "aliases_go121.go",
        "aliases_go122.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/internal/aliases",
    importpath = "golang.org/x/tools/internal/aliases",
    visibility = ["//vendor/golang.org/x/tools:__subpackages__"],
)
