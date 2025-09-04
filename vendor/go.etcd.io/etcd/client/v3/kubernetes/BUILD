load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "kubernetes",
    srcs = [
        "client.go",
        "interface.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.etcd.io/etcd/client/v3/kubernetes",
    importpath = "go.etcd.io/etcd/client/v3/kubernetes",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.etcd.io/etcd/api/v3/etcdserverpb",
        "//vendor/go.etcd.io/etcd/api/v3/mvccpb",
        "//vendor/go.etcd.io/etcd/client/v3:client",
    ],
)
