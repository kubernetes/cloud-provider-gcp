load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "schema",
    srcs = [
        "doc.go",
        "elements.go",
        "equals.go",
        "schemaschema.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/structured-merge-diff/v6/schema",
    importpath = "sigs.k8s.io/structured-merge-diff/v6/schema",
    visibility = ["//visibility:public"],
)
