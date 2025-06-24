load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "functions",
    srcs = ["functions.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/common/functions",
    importpath = "github.com/google/cel-go/common/functions",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/google/cel-go/common/types/ref"],
)
