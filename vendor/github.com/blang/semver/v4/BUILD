load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "semver",
    srcs = [
        "json.go",
        "range.go",
        "semver.go",
        "sort.go",
        "sql.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/blang/semver/v4",
    importpath = "github.com/blang/semver/v4",
    visibility = ["//visibility:public"],
)
