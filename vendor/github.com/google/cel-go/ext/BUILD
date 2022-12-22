load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "ext",
    srcs = [
        "encoders.go",
        "guards.go",
        "strings.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/ext",
    importpath = "github.com/google/cel-go/ext",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/cel",
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/ref",
    ],
)
