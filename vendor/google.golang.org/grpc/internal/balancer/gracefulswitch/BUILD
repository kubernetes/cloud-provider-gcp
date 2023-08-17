load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "gracefulswitch",
    srcs = ["gracefulswitch.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/internal/balancer/gracefulswitch",
    importpath = "google.golang.org/grpc/internal/balancer/gracefulswitch",
    visibility = ["//vendor/google.golang.org/grpc:__subpackages__"],
    deps = [
        "//vendor/google.golang.org/grpc/balancer",
        "//vendor/google.golang.org/grpc/balancer/base",
        "//vendor/google.golang.org/grpc/connectivity",
        "//vendor/google.golang.org/grpc/resolver",
    ],
)
