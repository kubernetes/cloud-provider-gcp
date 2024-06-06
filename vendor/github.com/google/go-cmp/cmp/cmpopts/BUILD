load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cmpopts",
    srcs = [
        "equate.go",
        "ignore.go",
        "sort.go",
        "struct_filter.go",
        "xform.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/go-cmp/cmp/cmpopts",
    importpath = "github.com/google/go-cmp/cmp/cmpopts",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/go-cmp/cmp",
        "//vendor/github.com/google/go-cmp/cmp/internal/function",
    ],
)
