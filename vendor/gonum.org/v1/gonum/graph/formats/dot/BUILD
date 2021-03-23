load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "dot.go",
        "sem.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gonum.org/v1/gonum/graph/formats/dot",
    importpath = "gonum.org/v1/gonum/graph/formats/dot",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/gonum.org/v1/gonum/graph/formats/dot/ast:go_default_library",
        "//vendor/gonum.org/v1/gonum/graph/formats/dot/internal/lexer:go_default_library",
        "//vendor/gonum.org/v1/gonum/graph/formats/dot/internal/parser:go_default_library",
    ],
)
