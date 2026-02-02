load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cover",
    srcs = ["profile.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/cover",
    importpath = "golang.org/x/tools/cover",
    visibility = ["//visibility:public"],
)
