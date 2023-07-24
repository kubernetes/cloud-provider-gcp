load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "decls",
    srcs = [
        "decls.go",
        "scopes.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/checker/decls",
    importpath = "github.com/google/cel-go/checker/decls",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/genproto/googleapis/api/expr/v1alpha1",
        "//vendor/google.golang.org/protobuf/types/known/emptypb",
        "//vendor/google.golang.org/protobuf/types/known/structpb",
    ],
)
