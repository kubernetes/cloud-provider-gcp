load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = ["version.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/googleapis/gax-go/v2/internal",
    importpath = "github.com/googleapis/gax-go/v2/internal",
    visibility = ["//vendor/github.com/googleapis/gax-go/v2:__subpackages__"],
)
