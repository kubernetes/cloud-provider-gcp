load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metadata",
    srcs = ["metadata.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/internal/metadata",
    importpath = "google.golang.org/grpc/internal/metadata",
    visibility = ["//vendor/google.golang.org/grpc:__subpackages__"],
    deps = [
        "//vendor/google.golang.org/grpc/metadata",
        "//vendor/google.golang.org/grpc/resolver",
    ],
)
