load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "journal.go",
        "journal_unix.go",
        "journal_windows.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/coreos/go-systemd/v22/journal",
    importpath = "github.com/coreos/go-systemd/v22/journal",
    visibility = ["//visibility:public"],
)
