load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "bidi",
    srcs = [
        "bidi.go",
        "bracket.go",
        "core.go",
        "prop.go",
        "tables15.0.0.go",
        "tables17.0.0.go",
        "trieval.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/text/unicode/bidi",
    importpath = "golang.org/x/text/unicode/bidi",
    visibility = ["//visibility:public"],
)
