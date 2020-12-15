load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "floats.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gonum.org/v1/gonum/floats",
    importpath = "gonum.org/v1/gonum/floats",
    visibility = ["//visibility:public"],
    deps = ["//vendor/gonum.org/v1/gonum/internal/asm/f64:go_default_library"],
)
