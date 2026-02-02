load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "slim-sprig",
    srcs = [
        "crypto.go",
        "date.go",
        "defaults.go",
        "dict.go",
        "doc.go",
        "functions.go",
        "list.go",
        "network.go",
        "numeric.go",
        "reflect.go",
        "regex.go",
        "strings.go",
        "url.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/go-task/slim-sprig/v3",
    importpath = "github.com/go-task/slim-sprig/v3",
    visibility = ["//visibility:public"],
)
