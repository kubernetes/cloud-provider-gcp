load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "lumberjack_v2",
    srcs = [
        "chown.go",
        "chown_linux.go",
        "lumberjack.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gopkg.in/natefinch/lumberjack.v2",
    importpath = "gopkg.in/natefinch/lumberjack.v2",
    visibility = ["//visibility:public"],
)
