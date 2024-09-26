load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "backoff",
    srcs = ["backoff.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/internal/backoff",
    importpath = "google.golang.org/grpc/internal/backoff",
    visibility = ["//vendor/google.golang.org/grpc:__subpackages__"],
    deps = ["//vendor/google.golang.org/grpc/backoff"],
)
