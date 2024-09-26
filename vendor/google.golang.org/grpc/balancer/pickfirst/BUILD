load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "pickfirst",
    srcs = ["pickfirst.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/balancer/pickfirst",
    importpath = "google.golang.org/grpc/balancer/pickfirst",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc/balancer",
        "//vendor/google.golang.org/grpc/connectivity",
        "//vendor/google.golang.org/grpc/grpclog",
        "//vendor/google.golang.org/grpc/internal",
        "//vendor/google.golang.org/grpc/internal/grpclog",
        "//vendor/google.golang.org/grpc/internal/pretty",
        "//vendor/google.golang.org/grpc/resolver",
        "//vendor/google.golang.org/grpc/serviceconfig",
    ],
)
