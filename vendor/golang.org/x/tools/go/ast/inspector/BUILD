load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "inspector",
    srcs = [
        "cursor.go",
        "inspector.go",
        "iter.go",
        "typeof.go",
        "walk.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/go/ast/inspector",
    importpath = "golang.org/x/tools/go/ast/inspector",
    visibility = ["//visibility:public"],
    deps = ["//vendor/golang.org/x/tools/go/ast/edge"],
)
