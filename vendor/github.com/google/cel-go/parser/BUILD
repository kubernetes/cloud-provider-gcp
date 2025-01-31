load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "parser",
    srcs = [
        "errors.go",
        "helper.go",
        "input.go",
        "macro.go",
        "options.go",
        "parser.go",
        "unescape.go",
        "unparser.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/parser",
    importpath = "github.com/google/cel-go/parser",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/antlr4-go/antlr/v4:antlr",
        "//vendor/github.com/google/cel-go/common",
        "//vendor/github.com/google/cel-go/common/ast",
        "//vendor/github.com/google/cel-go/common/operators",
        "//vendor/github.com/google/cel-go/common/runes",
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/github.com/google/cel-go/parser/gen",
    ],
)
