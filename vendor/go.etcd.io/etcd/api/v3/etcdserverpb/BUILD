load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "etcdserver.pb.go",
        "raft_internal.pb.go",
        "raft_internal_stringer.go",
        "rpc.pb.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.etcd.io/etcd/api/v3/etcdserverpb",
    importpath = "go.etcd.io/etcd/api/v3/etcdserverpb",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/gogo/protobuf/gogoproto:go_default_library",
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/go.etcd.io/etcd/api/v3/authpb:go_default_library",
        "//vendor/go.etcd.io/etcd/api/v3/membershippb:go_default_library",
        "//vendor/go.etcd.io/etcd/api/v3/mvccpb:go_default_library",
        "//vendor/google.golang.org/genproto/googleapis/api/annotations:go_default_library",
        "//vendor/google.golang.org/grpc:go_default_library",
        "//vendor/google.golang.org/grpc/codes:go_default_library",
        "//vendor/google.golang.org/grpc/status:go_default_library",
    ],
)
