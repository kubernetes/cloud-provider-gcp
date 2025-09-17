load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "env",
    srcs = ["env.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/common/env",
    importpath = "github.com/google/cel-go/common/env",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/common/decls",
        "//vendor/github.com/google/cel-go/common/types",
    ],
)
