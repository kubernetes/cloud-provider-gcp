load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "errors.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gonum.org/v1/gonum/graph/formats/dot/internal/errors",
    importpath = "gonum.org/v1/gonum/graph/formats/dot/internal/errors",
    visibility = ["//vendor/gonum.org/v1/gonum/graph/formats/dot:__subpackages__"],
    deps = ["//vendor/gonum.org/v1/gonum/graph/formats/dot/internal/token:go_default_library"],
)
