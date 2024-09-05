load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "rate",
    srcs = [
        "rate.go",
        "sometimes.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/time/rate",
    importpath = "golang.org/x/time/rate",
    visibility = ["//visibility:public"],
)
