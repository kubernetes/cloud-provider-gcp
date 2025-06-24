load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fallback",
    srcs = ["s2a_fallback.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/s2a-go/fallback",
    importpath = "github.com/google/s2a-go/fallback",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc/credentials",
        "//vendor/google.golang.org/grpc/grpclog",
    ],
)
