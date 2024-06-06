load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "roundrobin",
    srcs = ["roundrobin.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/balancer/roundrobin",
    importpath = "google.golang.org/grpc/balancer/roundrobin",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc/balancer",
        "//vendor/google.golang.org/grpc/balancer/base",
        "//vendor/google.golang.org/grpc/grpclog",
        "//vendor/google.golang.org/grpc/internal/grpcrand",
    ],
)
