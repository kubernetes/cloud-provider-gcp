load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = ["internal.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/cloud.google.com/go/auth/internal",
    importpath = "cloud.google.com/go/auth/internal",
    visibility = ["//vendor/cloud.google.com/go/auth:__subpackages__"],
    deps = ["//vendor/cloud.google.com/go/compute/metadata"],
)
