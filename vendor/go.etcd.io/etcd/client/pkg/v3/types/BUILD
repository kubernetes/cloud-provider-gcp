load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "id.go",
        "set.go",
        "slice.go",
        "urls.go",
        "urlsmap.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.etcd.io/etcd/client/pkg/v3/types",
    importpath = "go.etcd.io/etcd/client/pkg/v3/types",
    visibility = ["//visibility:public"],
)
