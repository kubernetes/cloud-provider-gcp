load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = ["compression.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/prometheus/client_golang/prometheus/promhttp/internal",
    importpath = "github.com/prometheus/client_golang/prometheus/promhttp/internal",
    visibility = ["//vendor/github.com/prometheus/client_golang/prometheus/promhttp:__subpackages__"],
)
