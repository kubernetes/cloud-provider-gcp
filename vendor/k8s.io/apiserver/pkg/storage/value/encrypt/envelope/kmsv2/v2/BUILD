load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "kmsv2",
    srcs = [
        "api.pb.go",
        "v2.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/kmsv2/v2",
    importpath = "k8s.io/apiserver/pkg/storage/value/encrypt/envelope/kmsv2/v2",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl",
    ],
)
