load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "ast",
    srcs = [
        "ast.go",
        "expr.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/common/ast",
    importpath = "github.com/google/cel-go/common/ast",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/google.golang.org/genproto/googleapis/api/expr/v1alpha1",
        "//vendor/google.golang.org/protobuf/types/known/structpb",
    ],
)
