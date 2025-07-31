load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "impersonate",
    srcs = ["impersonate.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/cloud.google.com/go/auth/credentials/internal/impersonate",
    importpath = "cloud.google.com/go/auth/credentials/internal/impersonate",
    visibility = ["//vendor/cloud.google.com/go/auth/credentials:__subpackages__"],
    deps = [
        "//vendor/cloud.google.com/go/auth",
        "//vendor/cloud.google.com/go/auth/internal",
    ],
)
