load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "api.pb.go",
        "v2alpha1.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/kmsv2/v2alpha1",
    importpath = "k8s.io/apiserver/pkg/storage/value/encrypt/envelope/kmsv2/v2alpha1",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/gogo/protobuf/proto:go_default_library"],
)
