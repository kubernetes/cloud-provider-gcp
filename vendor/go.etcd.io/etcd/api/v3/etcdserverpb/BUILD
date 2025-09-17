load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "etcdserverpb",
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
        "//vendor/github.com/gogo/protobuf/gogoproto",
        "//vendor/github.com/golang/protobuf/proto",
        "//vendor/github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options",
        "//vendor/go.etcd.io/etcd/api/v3/authpb",
        "//vendor/go.etcd.io/etcd/api/v3/membershippb",
        "//vendor/go.etcd.io/etcd/api/v3/mvccpb",
        "//vendor/go.etcd.io/etcd/api/v3/versionpb",
        "//vendor/google.golang.org/genproto/googleapis/api/annotations",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/status",
    ],
)
