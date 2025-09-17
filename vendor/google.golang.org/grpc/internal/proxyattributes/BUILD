load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "proxyattributes",
    srcs = ["proxyattributes.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/internal/proxyattributes",
    importpath = "google.golang.org/grpc/internal/proxyattributes",
    visibility = ["//vendor/google.golang.org/grpc:__subpackages__"],
    deps = ["//vendor/google.golang.org/grpc/resolver"],
)
