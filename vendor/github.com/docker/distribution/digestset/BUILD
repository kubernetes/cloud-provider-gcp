load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "digestset",
    srcs = ["set.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/docker/distribution/digestset",
    importpath = "github.com/docker/distribution/digestset",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/opencontainers/go-digest"],
)
