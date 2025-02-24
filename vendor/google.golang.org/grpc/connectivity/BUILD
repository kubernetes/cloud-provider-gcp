load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "connectivity",
    srcs = ["connectivity.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/connectivity",
    importpath = "google.golang.org/grpc/connectivity",
    visibility = ["//visibility:public"],
    deps = ["//vendor/google.golang.org/grpc/grpclog"],
)
