load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "joint.go",
        "majority.go",
        "quorum.go",
        "voteresult_string.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.etcd.io/etcd/raft/quorum",
    importpath = "go.etcd.io/etcd/raft/quorum",
    visibility = ["//visibility:public"],
)
