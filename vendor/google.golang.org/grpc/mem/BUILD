load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "mem",
    srcs = [
        "buffer_pool.go",
        "buffer_slice.go",
        "buffers.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/mem",
    importpath = "google.golang.org/grpc/mem",
    visibility = ["//visibility:public"],
    deps = ["//vendor/google.golang.org/grpc/internal"],
)
