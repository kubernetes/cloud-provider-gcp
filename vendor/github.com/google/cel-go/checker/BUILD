load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "checker",
    srcs = [
        "checker.go",
        "cost.go",
        "env.go",
        "errors.go",
        "mapping.go",
        "options.go",
        "printer.go",
        "standard.go",
        "types.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/checker",
    importpath = "github.com/google/cel-go/checker",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/checker/decls",
        "//vendor/github.com/google/cel-go/common",
        "//vendor/github.com/google/cel-go/common/containers",
        "//vendor/github.com/google/cel-go/common/debug",
        "//vendor/github.com/google/cel-go/common/operators",
        "//vendor/github.com/google/cel-go/common/overloads",
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/pb",
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/github.com/google/cel-go/parser",
        "//vendor/google.golang.org/genproto/googleapis/api/expr/v1alpha1",
        "//vendor/google.golang.org/protobuf/proto",
    ],
)
