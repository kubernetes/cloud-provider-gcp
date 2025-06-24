load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "decls",
    srcs = ["decls.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/common/decls",
    importpath = "github.com/google/cel-go/common/decls",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/checker/decls",
        "//vendor/github.com/google/cel-go/common/functions",
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/google.golang.org/genproto/googleapis/api/expr/v1alpha1",
    ],
)
