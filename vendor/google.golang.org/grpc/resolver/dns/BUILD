load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "dns",
    srcs = ["dns_resolver.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/resolver/dns",
    importpath = "google.golang.org/grpc/resolver/dns",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc/internal/resolver/dns",
        "//vendor/google.golang.org/grpc/resolver",
    ],
)
