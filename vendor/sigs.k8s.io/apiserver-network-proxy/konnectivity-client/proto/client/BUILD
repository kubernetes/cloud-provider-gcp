load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "client",
    srcs = ["client.pb.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/apiserver-network-proxy/konnectivity-client/proto/client",
    importpath = "sigs.k8s.io/apiserver-network-proxy/konnectivity-client/proto/client",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/status",
    ],
)
