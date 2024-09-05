load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "backoff",
    srcs = [
        "backoff.go",
        "context.go",
        "exponential.go",
        "retry.go",
        "ticker.go",
        "timer.go",
        "tries.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/cenkalti/backoff/v4",
    importpath = "github.com/cenkalti/backoff/v4",
    visibility = ["//visibility:public"],
)
