load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "protoregistry",
    srcs = ["registry.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/protobuf/reflect/protoregistry",
    importpath = "google.golang.org/protobuf/reflect/protoregistry",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/protobuf/internal/encoding/messageset",
        "//vendor/google.golang.org/protobuf/internal/errors",
        "//vendor/google.golang.org/protobuf/internal/flags",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
    ],
)
