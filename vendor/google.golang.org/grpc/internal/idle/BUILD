load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "idle",
    srcs = ["idle.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/internal/idle",
    importpath = "google.golang.org/grpc/internal/idle",
    visibility = ["//vendor/google.golang.org/grpc:__subpackages__"],
)
