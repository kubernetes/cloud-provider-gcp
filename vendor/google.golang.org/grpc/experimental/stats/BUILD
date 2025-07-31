load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "stats",
    srcs = [
        "metricregistry.go",
        "metrics.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/experimental/stats",
    importpath = "google.golang.org/grpc/experimental/stats",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc/grpclog",
        "//vendor/google.golang.org/grpc/internal",
    ],
)
