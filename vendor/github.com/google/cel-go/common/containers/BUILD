load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "containers",
    srcs = ["container.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/common/containers",
    importpath = "github.com/google/cel-go/common/containers",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/google/cel-go/common/ast"],
)
