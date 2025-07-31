load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "transport",
    srcs = [
        "cba.go",
        "s2a.go",
        "transport.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/cloud.google.com/go/auth/internal/transport",
    importpath = "cloud.google.com/go/auth/internal/transport",
    visibility = ["//vendor/cloud.google.com/go/auth:__subpackages__"],
    deps = [
        "//vendor/cloud.google.com/go/auth/credentials",
        "//vendor/cloud.google.com/go/auth/internal",
        "//vendor/cloud.google.com/go/auth/internal/transport/cert",
        "//vendor/cloud.google.com/go/compute/metadata",
        "//vendor/github.com/google/s2a-go",
        "//vendor/github.com/google/s2a-go/fallback",
        "//vendor/google.golang.org/grpc/credentials",
    ],
)
