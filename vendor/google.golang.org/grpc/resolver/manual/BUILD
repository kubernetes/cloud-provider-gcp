load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "manual",
    srcs = ["manual.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/resolver/manual",
    importpath = "google.golang.org/grpc/resolver/manual",
    visibility = ["//visibility:public"],
    deps = ["//vendor/google.golang.org/grpc/resolver"],
)
