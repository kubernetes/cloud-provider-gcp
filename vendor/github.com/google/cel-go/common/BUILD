load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "common",
    srcs = [
        "cost.go",
        "doc.go",
        "error.go",
        "errors.go",
        "location.go",
        "source.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/common",
    importpath = "github.com/google/cel-go/common",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/common/runes",
        "//vendor/google.golang.org/genproto/googleapis/api/expr/v1alpha1",
    ],
)
