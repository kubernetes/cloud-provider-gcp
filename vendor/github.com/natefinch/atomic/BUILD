load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "atomic",
    srcs = [
        "atomic.go",
        "file_unix.go",
        "file_windows.go",
        "zfile_windows.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/natefinch/atomic",
    importpath = "github.com/natefinch/atomic",
    visibility = ["//visibility:public"],
)
