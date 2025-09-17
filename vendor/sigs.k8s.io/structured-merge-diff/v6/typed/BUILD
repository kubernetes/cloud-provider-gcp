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
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/structured-merge-diff/v6/typed",
    importpath = "sigs.k8s.io/structured-merge-diff/v6/typed",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.yaml.in/yaml/v2:yaml",
        "//vendor/sigs.k8s.io/structured-merge-diff/v6/fieldpath",
        "//vendor/sigs.k8s.io/structured-merge-diff/v6/schema",
        "//vendor/sigs.k8s.io/structured-merge-diff/v6/value",
    ],
)
