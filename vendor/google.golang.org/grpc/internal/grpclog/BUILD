load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "grpclog",
    srcs = ["prefix_logger.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/internal/grpclog",
    importpath = "google.golang.org/grpc/internal/grpclog",
    visibility = ["//vendor/google.golang.org/grpc:__subpackages__"],
    deps = ["//vendor/google.golang.org/grpc/grpclog"],
)
