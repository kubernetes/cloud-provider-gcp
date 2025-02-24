load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "jwt",
    srcs = ["jwt.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/cloud.google.com/go/auth/internal/jwt",
    importpath = "cloud.google.com/go/auth/internal/jwt",
    visibility = ["//vendor/cloud.google.com/go/auth:__subpackages__"],
)
