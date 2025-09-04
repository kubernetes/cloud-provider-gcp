load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "html",
    srcs = [
        "const.go",
        "doc.go",
        "doctype.go",
        "entity.go",
        "escape.go",
        "foreign.go",
        "iter.go",
        "node.go",
        "parse.go",
        "render.go",
        "token.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/net/html",
    importpath = "golang.org/x/net/html",
    visibility = ["//visibility:public"],
    deps = ["//vendor/golang.org/x/net/html/atom"],
)
