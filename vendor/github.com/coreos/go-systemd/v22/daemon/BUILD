load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "daemon",
    srcs = [
        "sdnotify.go",
        "watchdog.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/coreos/go-systemd/v22/daemon",
    importpath = "github.com/coreos/go-systemd/v22/daemon",
    visibility = ["//visibility:public"],
)
