load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = [
        "internal.go",
        "xds_handshake_cluster.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/internal",
    importpath = "google.golang.org/grpc/internal",
    visibility = ["//vendor/google.golang.org/grpc:__subpackages__"],
    deps = [
        "//vendor/google.golang.org/grpc/attributes",
        "//vendor/google.golang.org/grpc/connectivity",
        "//vendor/google.golang.org/grpc/resolver",
        "//vendor/google.golang.org/grpc/serviceconfig",
    ],
)
