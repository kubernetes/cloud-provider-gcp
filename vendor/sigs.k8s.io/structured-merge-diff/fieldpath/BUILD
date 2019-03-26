load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "element.go",
        "fromvalue.go",
        "managers.go",
        "path.go",
        "set.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/structured-merge-diff/fieldpath",
    importpath = "sigs.k8s.io/structured-merge-diff/fieldpath",
    visibility = ["//visibility:public"],
    deps = ["//vendor/sigs.k8s.io/structured-merge-diff/value:go_default_library"],
)
