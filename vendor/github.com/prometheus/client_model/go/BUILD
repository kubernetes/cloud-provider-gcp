load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go",
    srcs = ["metrics.pb.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/prometheus/client_model/go",
    importpath = "github.com/prometheus/client_model/go",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl",
        "//vendor/google.golang.org/protobuf/types/known/timestamppb",
    ],
)
