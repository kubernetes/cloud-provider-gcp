load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "merge",
    srcs = [
        "conflict.go",
        "update.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/structured-merge-diff/v6/merge",
    importpath = "sigs.k8s.io/structured-merge-diff/v6/merge",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/sigs.k8s.io/structured-merge-diff/v6/fieldpath",
        "//vendor/sigs.k8s.io/structured-merge-diff/v6/typed",
        "//vendor/sigs.k8s.io/structured-merge-diff/v6/value",
    ],
)
