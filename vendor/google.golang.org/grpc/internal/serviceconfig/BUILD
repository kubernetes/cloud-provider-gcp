load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "serviceconfig",
    srcs = ["serviceconfig.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/internal/serviceconfig",
    importpath = "google.golang.org/grpc/internal/serviceconfig",
    visibility = ["//vendor/google.golang.org/grpc:__subpackages__"],
    deps = [
        "//vendor/google.golang.org/grpc/balancer",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/grpclog",
        "//vendor/google.golang.org/grpc/serviceconfig",
    ],
)
