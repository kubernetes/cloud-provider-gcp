load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "action.go",
        "actiontable.go",
        "doc.go",
        "gototable.go",
        "parser.go",
        "productionstable.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gonum.org/v1/gonum/graph/formats/dot/internal/parser",
    importpath = "gonum.org/v1/gonum/graph/formats/dot/internal/parser",
    visibility = ["//vendor/gonum.org/v1/gonum/graph/formats/dot:__subpackages__"],
    deps = [
        "//vendor/gonum.org/v1/gonum/graph/formats/dot/ast:go_default_library",
        "//vendor/gonum.org/v1/gonum/graph/formats/dot/internal/astx:go_default_library",
        "//vendor/gonum.org/v1/gonum/graph/formats/dot/internal/errors:go_default_library",
        "//vendor/gonum.org/v1/gonum/graph/formats/dot/internal/token:go_default_library",
    ],
)
