load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "btree",
    srcs = [
        "btree.go",
        "btree_generic.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/btree",
    importpath = "github.com/google/btree",
    visibility = ["//visibility:public"],
)
