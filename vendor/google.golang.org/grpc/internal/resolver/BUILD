load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "resolver",
    srcs = ["config_selector.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/internal/resolver",
    importpath = "google.golang.org/grpc/internal/resolver",
    visibility = ["//vendor/google.golang.org/grpc:__subpackages__"],
    deps = [
        "//vendor/google.golang.org/grpc/internal/serviceconfig",
        "//vendor/google.golang.org/grpc/resolver",
    ],
)
