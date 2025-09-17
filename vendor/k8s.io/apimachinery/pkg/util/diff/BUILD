load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "diff",
    srcs = [
        "diff.go",
        "legacy_diff.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/util/diff",
    importpath = "k8s.io/apimachinery/pkg/util/diff",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/pmezard/go-difflib/difflib",
        "//vendor/k8s.io/apimachinery/pkg/util/dump",
    ],
)
