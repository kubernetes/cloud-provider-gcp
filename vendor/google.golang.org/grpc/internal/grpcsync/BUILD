load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "grpcsync",
    srcs = [
        "callback_serializer.go",
        "event.go",
        "pubsub.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/internal/grpcsync",
    importpath = "google.golang.org/grpc/internal/grpcsync",
    visibility = ["//vendor/google.golang.org/grpc:__subpackages__"],
    deps = ["//vendor/google.golang.org/grpc/internal/buffer"],
)
