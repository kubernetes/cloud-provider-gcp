load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "mvccpb",
    srcs = ["kv.pb.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.etcd.io/etcd/api/v3/mvccpb",
    importpath = "go.etcd.io/etcd/api/v3/mvccpb",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/gogo/protobuf/gogoproto",
        "//vendor/github.com/golang/protobuf/proto",
    ],
)
