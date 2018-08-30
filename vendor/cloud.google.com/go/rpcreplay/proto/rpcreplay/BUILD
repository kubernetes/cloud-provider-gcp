load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["rpcreplay.pb.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/rpcreplay/proto/rpcreplay",
    importpath = "cloud.google.com/go/rpcreplay/proto/rpcreplay",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "@io_bazel_rules_go//proto/wkt:any_go_proto",
    ],
)
