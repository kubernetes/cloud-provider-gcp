load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "debug",
    srcs = ["debug.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/common/debug",
    importpath = "github.com/google/cel-go/common/debug",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/common/ast",
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/ref",
    ],
)
