load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "retry",
    srcs = ["retry.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/s2a-go/retry",
    importpath = "github.com/google/s2a-go/retry",
    visibility = ["//visibility:public"],
    deps = ["//vendor/google.golang.org/grpc/grpclog"],
)
