load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "endpointsharding",
    srcs = ["endpointsharding.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/balancer/endpointsharding",
    importpath = "google.golang.org/grpc/balancer/endpointsharding",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc/balancer",
        "//vendor/google.golang.org/grpc/balancer/base",
        "//vendor/google.golang.org/grpc/connectivity",
        "//vendor/google.golang.org/grpc/resolver",
    ],
)
