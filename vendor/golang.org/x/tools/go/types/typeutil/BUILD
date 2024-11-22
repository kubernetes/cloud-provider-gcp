load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "typeutil",
    srcs = [
        "callee.go",
        "imports.go",
        "map.go",
        "methodsetcache.go",
        "ui.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/go/types/typeutil",
    importpath = "golang.org/x/tools/go/types/typeutil",
    visibility = ["//visibility:public"],
    deps = ["//vendor/golang.org/x/tools/internal/typeparams"],
)
