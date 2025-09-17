load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "versionpb",
    srcs = ["version.pb.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.etcd.io/etcd/api/v3/versionpb",
    importpath = "go.etcd.io/etcd/api/v3/versionpb",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/gogo/protobuf/gogoproto",
        "//vendor/github.com/gogo/protobuf/protoc-gen-gogo/descriptor",
        "//vendor/github.com/golang/protobuf/proto",
    ],
)
