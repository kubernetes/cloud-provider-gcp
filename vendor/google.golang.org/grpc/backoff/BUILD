load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "backoff",
    srcs = ["backoff.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/backoff",
    importpath = "google.golang.org/grpc/backoff",
    visibility = ["//visibility:public"],
)
