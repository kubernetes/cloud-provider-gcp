load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "balancer",
    srcs = [
        "balancer.go",
        "conn_state_evaluator.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/balancer",
    importpath = "google.golang.org/grpc/balancer",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc/channelz",
        "//vendor/google.golang.org/grpc/connectivity",
        "//vendor/google.golang.org/grpc/credentials",
        "//vendor/google.golang.org/grpc/experimental/stats",
        "//vendor/google.golang.org/grpc/grpclog",
        "//vendor/google.golang.org/grpc/internal",
        "//vendor/google.golang.org/grpc/metadata",
        "//vendor/google.golang.org/grpc/resolver",
        "//vendor/google.golang.org/grpc/serviceconfig",
    ],
)
