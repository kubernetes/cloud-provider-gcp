load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "gcfg_v1",
    srcs = [
        "doc.go",
        "errors.go",
        "read.go",
        "set.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gopkg.in/gcfg.v1",
    importpath = "gopkg.in/gcfg.v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/gopkg.in/gcfg.v1/scanner",
        "//vendor/gopkg.in/gcfg.v1/token",
        "//vendor/gopkg.in/gcfg.v1/types",
        "//vendor/gopkg.in/warnings.v0:warnings_v0",
    ],
)
