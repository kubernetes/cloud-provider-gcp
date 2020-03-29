load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "journal.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.etcd.io/etcd/pkg/systemd",
    importpath = "go.etcd.io/etcd/pkg/systemd",
    visibility = ["//visibility:public"],
)
