load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "stats",
    srcs = [
        "labels.go",
        "metrics_recorder_list.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/internal/stats",
    importpath = "google.golang.org/grpc/internal/stats",
    visibility = ["//vendor/google.golang.org/grpc:__subpackages__"],
    deps = [
        "//vendor/google.golang.org/grpc/experimental/stats",
        "//vendor/google.golang.org/grpc/stats",
    ],
)
