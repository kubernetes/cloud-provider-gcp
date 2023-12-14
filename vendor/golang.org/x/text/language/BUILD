load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "language",
    srcs = [
        "coverage.go",
        "doc.go",
        "language.go",
        "match.go",
        "parse.go",
        "tables.go",
        "tags.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/text/language",
    importpath = "golang.org/x/text/language",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/text/internal/language",
        "//vendor/golang.org/x/text/internal/language/compact",
    ],
)
