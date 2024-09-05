load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "dns",
    srcs = ["dns_resolver.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/internal/resolver/dns",
    importpath = "google.golang.org/grpc/internal/resolver/dns",
    visibility = ["//vendor/google.golang.org/grpc:__subpackages__"],
    deps = [
        "//vendor/google.golang.org/grpc/balancer/grpclb/state",
        "//vendor/google.golang.org/grpc/grpclog",
        "//vendor/google.golang.org/grpc/internal/backoff",
        "//vendor/google.golang.org/grpc/internal/envconfig",
        "//vendor/google.golang.org/grpc/internal/grpcrand",
        "//vendor/google.golang.org/grpc/internal/resolver/dns/internal",
        "//vendor/google.golang.org/grpc/resolver",
        "//vendor/google.golang.org/grpc/serviceconfig",
    ],
)
