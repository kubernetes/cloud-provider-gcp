load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "channelz",
    srcs = ["channelz.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/channelz",
    importpath = "google.golang.org/grpc/channelz",
    visibility = ["//visibility:public"],
    deps = ["//vendor/google.golang.org/grpc/internal/channelz"],
)
