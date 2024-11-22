load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "typed",
    srcs = [
        "compare.go",
        "doc.go",
        "helpers.go",
        "merge.go",
        "parser.go",
        "reconcile_schema.go",
        "remove.go",
        "tofieldset.go",
        "typed.go",
        "validate.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/structured-merge-diff/v4/typed",
    importpath = "sigs.k8s.io/structured-merge-diff/v4/typed",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/sigs.k8s.io/structured-merge-diff/v4/fieldpath",
        "//vendor/sigs.k8s.io/structured-merge-diff/v4/schema",
        "//vendor/sigs.k8s.io/structured-merge-diff/v4/value",
        "//vendor/sigs.k8s.io/yaml/goyaml.v2:goyaml_v2",
    ],
)
