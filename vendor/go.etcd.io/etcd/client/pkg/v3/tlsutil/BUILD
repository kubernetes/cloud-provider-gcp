load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "tlsutil",
    srcs = [
        "cipher_suites.go",
        "doc.go",
        "tlsutil.go",
        "versions.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.etcd.io/etcd/client/pkg/v3/tlsutil",
    importpath = "go.etcd.io/etcd/client/pkg/v3/tlsutil",
    visibility = ["//visibility:public"],
)
