load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "logr",
    srcs = [
        "context.go",
        "context_noslog.go",
        "context_slog.go",
        "discard.go",
        "logr.go",
        "sloghandler.go",
        "slogr.go",
        "slogsink.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/go-logr/logr",
    importpath = "github.com/go-logr/logr",
    visibility = ["//visibility:public"],
)
