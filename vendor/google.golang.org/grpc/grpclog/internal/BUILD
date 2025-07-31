load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = [
        "grpclog.go",
        "logger.go",
        "loggerv2.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/grpclog/internal",
    importpath = "google.golang.org/grpc/grpclog/internal",
    visibility = ["//vendor/google.golang.org/grpc/grpclog:__subpackages__"],
)
