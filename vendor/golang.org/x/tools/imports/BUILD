load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "imports",
    srcs = ["forward.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/imports",
    importpath = "golang.org/x/tools/imports",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/tools/internal/gocommand",
        "//vendor/golang.org/x/tools/internal/imports",
    ],
)
