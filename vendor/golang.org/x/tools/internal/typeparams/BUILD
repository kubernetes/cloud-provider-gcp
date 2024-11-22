load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "typeparams",
    srcs = [
        "common.go",
        "coretype.go",
        "free.go",
        "normalize.go",
        "termlist.go",
        "typeterm.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/internal/typeparams",
    importpath = "golang.org/x/tools/internal/typeparams",
    visibility = ["//vendor/golang.org/x/tools:__subpackages__"],
    deps = ["//vendor/golang.org/x/tools/internal/aliases"],
)
