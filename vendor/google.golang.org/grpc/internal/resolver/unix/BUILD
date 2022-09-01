load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "unix",
    srcs = ["unix.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/internal/resolver/unix",
    importpath = "google.golang.org/grpc/internal/resolver/unix",
    visibility = ["//vendor/google.golang.org/grpc:__subpackages__"],
    deps = [
        "//vendor/google.golang.org/grpc/internal/transport/networktype",
        "//vendor/google.golang.org/grpc/resolver",
    ],
)
