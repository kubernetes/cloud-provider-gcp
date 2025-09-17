load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "pickfirstleaf",
    srcs = ["pickfirstleaf.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/balancer/pickfirst/pickfirstleaf",
    importpath = "google.golang.org/grpc/balancer/pickfirst/pickfirstleaf",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc/balancer",
        "//vendor/google.golang.org/grpc/balancer/pickfirst/internal",
        "//vendor/google.golang.org/grpc/connectivity",
        "//vendor/google.golang.org/grpc/experimental/stats",
        "//vendor/google.golang.org/grpc/grpclog",
        "//vendor/google.golang.org/grpc/internal/envconfig",
        "//vendor/google.golang.org/grpc/internal/grpclog",
        "//vendor/google.golang.org/grpc/internal/pretty",
        "//vendor/google.golang.org/grpc/resolver",
        "//vendor/google.golang.org/grpc/serviceconfig",
    ],
)
