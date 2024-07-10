load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "resolver",
    srcs = [
        "map.go",
        "resolver.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/resolver",
    importpath = "google.golang.org/grpc/resolver",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc/attributes",
        "//vendor/google.golang.org/grpc/credentials",
        "//vendor/google.golang.org/grpc/internal",
        "//vendor/google.golang.org/grpc/serviceconfig",
    ],
)
